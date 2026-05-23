// cSpell: words specv oras VMVHDX vhdx hyperv qcow2 artifacttype gochecknoglobals VMQCOW
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	specv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

// ArtifactType identifies expected image artifact content.
type ArtifactType string

const (
	ArtifactRootFS  ArtifactType = "rootfs"
	ArtifactVMVHDX  ArtifactType = "vm-vhdx"
	ArtifactVMQCOW2 ArtifactType = "vm-qcow2"

	defaultImageTag = "latest"

	vhdxArtifactType  = "application/vnd.oci.image.layer.vhdx"
	qcow2ArtifactType = "application/vnd.oci.image.layer.qcow2"

	rootfsMediaTypeDocker = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	rootfsMediaTypeOCI    = "application/vnd.oci.image.layer.v1.tar+gzip"
	vhdxMediaType         = "application/x-hyperv-disk"
	qcow2MediaType        = "application/x-qcow2"
	incusMetadataType     = "application/vnd.incus.metadata"

	inspectResultFileName = "inspect-result.json"
)

//nolint:gochecknoglobals // Compiled regex reused by helper.
var imageDirSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Repository exposes minimal registry operations for DI/testability.
type Repository interface {
	Resolve(ctx context.Context, reference string) (specv1.Descriptor, error)
	Fetch(ctx context.Context, target specv1.Descriptor) (io.ReadCloser, error)
}

// NewRepositoryFunc creates a repository client.
type NewRepositoryFunc func(repository string) (Repository, error)

// Service provides image inspect and pull operations.
type Service struct {
	FS            host.FileEnvironment
	NewRepository NewRepositoryFunc
}

// InspectResult contains parsed manifest details.
type InspectResult struct {
	Manifest         specv1.Manifest
	Descriptor       specv1.Descriptor
	Repository       string
	Reference        string
	ImageType        ArtifactType
	ManifestTypeHint string
}

// PullRequest defines image pull behavior.
type PullRequest struct {
	ImageRef string
}

func (s *Service) ensureDefaults() error {
	if s.FS == nil {
		return fmt.Errorf("filesystem dependency is required")
	}
	if s.NewRepository == nil {
		s.NewRepository = newRemoteRepository
	}

	return nil
}

// Inspect resolves and fetches manifest content for an image reference.
func (s *Service) Inspect(ctx context.Context, imageRef string) (*InspectResult, error) {
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	repository, reference := splitImageReference(imageRef)
	repo, err := s.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository client: %w", err)
	}

	desc, err := repo.Resolve(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve image reference: %w", err)
	}

	reader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}

	manifestBytes, err := io.ReadAll(reader)
	if err != nil {
		if closeErr := reader.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to close manifest reader after read error: %w", closeErr)
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	if closeErr := reader.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close manifest reader: %w", closeErr)
	}

	var manifest specv1.Manifest
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	imageType, err := inferImageType(&manifest)
	if err != nil {
		return nil, err
	}

	return &InspectResult{
		Repository:       repository,
		Reference:        reference,
		Descriptor:       desc,
		Manifest:         manifest,
		ImageType:        imageType,
		ManifestTypeHint: manifest.ArtifactType,
	}, nil
}

// Pull downloads image artifacts into ~/.config/iknite/images/<image-id>/ and writes inspect-result.json.
func (s *Service) Pull(ctx context.Context, req *PullRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("pull request is required")
	}
	if req.ImageRef == "" {
		return "", fmt.Errorf("image reference is required")
	}

	inspectResult, err := s.Inspect(ctx, req.ImageRef)
	if err != nil {
		return "", err
	}

	configDir, err := s.FS.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	outputDir := s.FS.JoinPath(configDir, constants.IkniteConfName, "images", imageDirectoryName(inspectResult))
	if err = s.FS.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	alreadyDownloaded, err := s.hasMatchingSavedManifest(outputDir, inspectResult)
	if err != nil {
		return "", err
	}
	if alreadyDownloaded {
		return outputDir, nil
	}

	layers, err := selectArtifactLayers(&inspectResult.Manifest, inspectResult.ImageType)
	if err != nil {
		return "", err
	}

	repo, err := s.NewRepository(inspectResult.Repository)
	if err != nil {
		return "", fmt.Errorf("failed to create repository client: %w", err)
	}

	for _, layer := range layers {
		blob, readErr := readBlob(ctx, repo, layer.Descriptor)
		if readErr != nil {
			return "", readErr
		}

		outputPath := filepath.Join(outputDir, layer.FileName)
		if writeErr := s.FS.WriteFile(outputPath, blob, 0o644); writeErr != nil {
			return "", fmt.Errorf("failed to write output file %s: %w", outputPath, writeErr)
		}
	}

	inspectJSON, err := json.MarshalIndent(inspectResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal inspect result: %w", err)
	}
	inspectPath := filepath.Join(outputDir, inspectResultFileName)
	if err = s.FS.WriteFile(inspectPath, inspectJSON, 0o644); err != nil {
		return "", fmt.Errorf("failed to write inspect result file: %w", err)
	}

	return outputDir, nil
}

func (s *Service) hasMatchingSavedManifest(outputDir string, inspectResult *InspectResult) (bool, error) {
	inspectPath := s.FS.JoinPath(outputDir, inspectResultFileName)
	if _, statErr := s.FS.Stat(inspectPath); os.IsNotExist(statErr) {
		return false, nil
	} else if statErr != nil {
		return false, fmt.Errorf("failed to stat saved inspect result: %w", statErr)
	}

	savedInspectRaw, err := s.FS.ReadFile(inspectPath)
	if err != nil {
		return false, fmt.Errorf("failed to read saved inspect result: %w", err)
	}

	var savedInspect InspectResult
	if unmarshalErr := json.Unmarshal(savedInspectRaw, &savedInspect); unmarshalErr == nil {
		savedManifest, err := json.Marshal(savedInspect.Manifest)
		if err != nil {
			return false, fmt.Errorf("failed to marshal saved manifest: %w", err)
		}

		fetchedManifest, err := json.Marshal(inspectResult.Manifest)
		if err != nil {
			return false, fmt.Errorf("failed to marshal fetched manifest: %w", err)
		}

		return bytes.Equal(savedManifest, fetchedManifest), nil
	}

	// Corrupted or legacy inspect file: refresh local artifacts.
	return false, nil
}

func inferImageType(manifest *specv1.Manifest) (ArtifactType, error) {
	if manifest.ArtifactType == "" && len(manifest.Layers) == 1 {
		layerType := manifest.Layers[0].MediaType
		if layerType == rootfsMediaTypeDocker || layerType == rootfsMediaTypeOCI {
			return ArtifactRootFS, nil
		}
		return "", fmt.Errorf("unsupported rootfs layer media type: %s", layerType)
	}

	if manifest.ArtifactType == vhdxArtifactType {
		if len(manifest.Layers) != 1 {
			return "", fmt.Errorf("vhdx image must have exactly one layer")
		}
		if manifest.Layers[0].MediaType != vhdxMediaType {
			return "", fmt.Errorf("vhdx image layer media type must be %s", vhdxMediaType)
		}
		return ArtifactVMVHDX, nil
	}

	if manifest.ArtifactType == qcow2ArtifactType {
		if len(manifest.Layers) != 2 {
			return "", fmt.Errorf("qcow2 image must have exactly two layers")
		}
		hasQCOW2 := false
		hasIncusMetadata := false
		for i := range manifest.Layers {
			switch manifest.Layers[i].MediaType {
			case qcow2MediaType:
				hasQCOW2 = true
			case incusMetadataType:
				hasIncusMetadata = true
			}
		}
		if !hasQCOW2 || !hasIncusMetadata {
			return "", fmt.Errorf("qcow2 image must contain qcow2 and incus metadata layers")
		}
		return ArtifactVMQCOW2, nil
	}

	return "", fmt.Errorf("unsupported manifest artifactType %q", manifest.ArtifactType)
}

type pullLayer struct {
	Descriptor *specv1.Descriptor
	FileName   string
}

func selectArtifactLayers(manifest *specv1.Manifest, imageType ArtifactType) ([]pullLayer, error) {
	result := make([]pullLayer, 0, len(manifest.Layers))

	switch imageType {
	case ArtifactRootFS:
		result = append(result, pullLayer{Descriptor: &manifest.Layers[0], FileName: "rootfs.tar.gz"})
		return result, nil
	case ArtifactVMVHDX:
		result = append(result, pullLayer{Descriptor: &manifest.Layers[0], FileName: "disk.vhdx"})
		return result, nil
	case ArtifactVMQCOW2:
		for i := range manifest.Layers {
			layer := &manifest.Layers[i]
			switch layer.MediaType {
			case qcow2MediaType:
				result = append(result, pullLayer{Descriptor: layer, FileName: "disk.qcow2"})
			case incusMetadataType:
				result = append(result, pullLayer{Descriptor: layer, FileName: "incus-metadata.bin"})
			}
		}
		if len(result) != 2 {
			return nil, fmt.Errorf("qcow2 image must provide both qcow2 and incus metadata layers")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported image type %s", imageType)
	}
}

func readBlob(ctx context.Context, repo Repository, layer *specv1.Descriptor) ([]byte, error) {
	blobReader, err := repo.Fetch(ctx, *layer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blob %s: %w", layer.Digest.String(), err)
	}

	blob, err := io.ReadAll(blobReader)
	if err != nil {
		if closeErr := blobReader.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to close blob reader after read error: %w", closeErr)
		}
		return nil, fmt.Errorf("failed to read blob data: %w", err)
	}
	if closeErr := blobReader.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed to close blob reader: %w", closeErr)
	}

	return blob, nil
}

func imageDirectoryName(result *InspectResult) string {
	raw := fmt.Sprintf("%s@%s", result.Repository, result.Reference)
	clean := imageDirSanitizer.ReplaceAllString(raw, "_")
	clean = strings.Trim(clean, "._-")
	if clean == "" {
		return "image"
	}

	return clean
}

func splitImageReference(imageRef string) (string, string) {
	if strings.Contains(imageRef, "@") {
		parts := strings.SplitN(imageRef, "@", 2)
		return parts[0], parts[1]
	}

	tag := defaultImageTag
	repository := imageRef
	slashIndex := strings.LastIndex(imageRef, "/")
	colonIndex := strings.LastIndex(imageRef, ":")
	if colonIndex > slashIndex {
		repository = imageRef[:colonIndex]
		tag = imageRef[colonIndex+1:]
	}

	return repository, tag
}

func newRemoteRepository(repository string) (Repository, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote repository: %w", err)
	}

	repo.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(repo.Reference.Registry, auth.EmptyCredential),
	}

	return repo, nil
}
