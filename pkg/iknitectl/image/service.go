// cSpell: words specv oras VMVHDX hyperv qcow2 artifacttype gochecknoglobals VMQCOW
package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"

	specv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
)

const (
	defaultImageTag = "latest"

	vhdxArtifactType  = "application/vnd.oci.image.layer.vhdx"
	qcow2ArtifactType = "application/vnd.oci.image.layer.qcow2"

	rootfsMediaTypeDocker = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	rootfsMediaTypeOCI    = "application/vnd.oci.image.layer.v1.tar+gzip"
	vhdxMediaType         = "application/x-hyperv-disk"
	qcow2MediaType        = "application/x-qcow2"
	incusMetadataType     = "application/vnd.incus.metadata"
	manifestFilename      = "manifest.json"
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

// MetadataStore exposes the subset of database operations needed by image pull.
type MetadataStore interface {
	GetItem(id string, out any) error
	CreateItem(item db.IDAccessor) error
	UpdateItem(item db.IDAccessor) error
	CreateOrUpdateItem(item db.IDAccessor) error
	DeleteItem(item db.IDAccessor) error
	ListItems(out any) error
	SetNameRef(name, ref string) error
	GetNameRef(name string) (string, error)
	RemoveNameRef(name string) error
}

// Service provides image inspect and pull operations.
type Service struct {
	FS            host.FileEnvironment
	Logger        *slog.Logger
	Config        *config.Config
	Store         MetadataStore
	NewRepository NewRepositoryFunc
}

func NewService(fs host.FileEnvironment, logger *slog.Logger, c *config.Config, s MetadataStore) *Service {
	return &Service{
		FS:            fs,
		Logger:        logger,
		Config:        c,
		Store:         s,
		NewRepository: newRemoteRepository,
	}
}

// PullLayer represents a single artifact layer to be pulled, along with its target filename.
type PullLayer struct {
	Descriptor *specv1.Descriptor
	FileName   string
}

// InspectResult contains parsed manifest details.
type InspectResult struct {
	Manifest         specv1.Manifest
	Descriptor       specv1.Descriptor
	Repository       string
	Reference        string
	ManifestTypeHint string
	Layers           []PullLayer
	ImageType        ImageType
}

// PullRequest defines image pull behavior.
type PullRequest struct {
	ImageRef string
}

func summarizeArtifacts(artifacts []db.ImageArtifact) string {
	if len(artifacts) == 0 {
		return ""
	}

	types := make([]string, 0, len(artifacts))
	for i := range artifacts {
		types = append(types, string(artifacts[i].Type))
	}
	sort.Strings(types)

	return fmt.Sprintf("%d [%s]", len(types), strings.Join(types, ", "))
}

func sumArtifactSizes(artifacts []db.ImageArtifact) int64 {
	var total int64
	for i := range artifacts {
		total += artifacts[i].Size
	}

	return total
}

// ListImages loads persisted image metadata and joins source, version, and artifact details.
func (s *Service) ListImages() ([]ImageListItem, error) {
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	images := make([]db.Image, 0)
	err := s.Store.ListItems(&images)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	versions := make([]db.ImageVersion, 0)
	err = s.Store.ListItems(&versions)
	if err != nil {
		return nil, fmt.Errorf("failed to list image versions: %w", err)
	}
	sources := make([]db.ImageSource, 0)
	err = s.Store.ListItems(&sources)
	if err != nil {
		return nil, fmt.Errorf("failed to list image sources: %w", err)
	}
	artifacts := make([]db.ImageArtifact, 0)
	err = s.Store.ListItems(&artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to list image artifacts: %w", err)
	}

	versionByID := make(map[string]db.ImageVersion, len(versions))
	for i := range versions {
		versionByID[versions[i].ID] = versions[i]
	}

	sourceByID := make(map[string]db.ImageSource, len(sources))
	for i := range sources {
		sourceByID[sources[i].ID] = sources[i]
	}

	artifactByImageID := make(map[string][]db.ImageArtifact)
	for i := range artifacts {
		artifact := artifacts[i]
		artifactByImageID[artifact.ImageID] = append(artifactByImageID[artifact.ImageID], artifact)
	}

	sort.Slice(images, func(i, j int) bool {
		if images[i].Name == images[j].Name {
			return images[i].ID < images[j].ID
		}
		return images[i].Name < images[j].Name
	})

	result := make([]ImageListItem, 0, len(images))
	for _, image := range images {
		version := versionByID[image.VersionID]
		source := sourceByID[version.SourceID]
		imageArtifacts := artifactByImageID[image.ID]
		result = append(result, ImageListItem{
			Name:      image.Name,
			Source:    source.Location,
			Reference: version.Tag,
			Path:      image.Path,
			Artifacts: summarizeArtifacts(imageArtifacts),
			TotalSize: sumArtifactSizes(imageArtifacts),
			UpdatedAt: image.UpdatedAt,
		})
	}

	return result, nil
}

func (s *Service) ensureDefaults() error {
	if s.FS == nil {
		return fmt.Errorf("filesystem dependency is required")
	}
	if s.Store == nil {
		return fmt.Errorf("store dependency is required")
	}

	if s.NewRepository == nil {
		s.NewRepository = newRemoteRepository
	}

	if s.Logger == nil {
		s.Logger = util.DefaultBaseOptions().Logger()
	}
	if s.Config == nil {
		s.Config = &config.Config{}
		if err := config.NewConfigOptions(s.FS).Resolve(s.FS, s.Config); err != nil {
			return fmt.Errorf("failed to resolve config options: %w", err)
		}
	}

	return nil
}

// Info returns extended information about a downloaded image identified by its display name.
func (s *Service) Info(imageName string) (*ImageInfo, error) {
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	// Resolve the image name to the version ID via the name-refs bucket.
	versionID, err := s.Store.GetNameRef(imageName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve image name %q: %w", imageName, err)
	}

	// Load the image record.
	var image db.Image
	if err = s.Store.GetItem(versionID, &image); err != nil {
		return nil, fmt.Errorf("failed to get image record: %w", err)
	}

	// Load the version record.
	var version db.ImageVersion
	if err = s.Store.GetItem(image.VersionID, &version); err != nil {
		return nil, fmt.Errorf("failed to get image version: %w", err)
	}

	// Load the source record.
	var source db.ImageSource
	if err = s.Store.GetItem(version.SourceID, &source); err != nil {
		return nil, fmt.Errorf("failed to get image source: %w", err)
	}

	// Load all artifacts for this image.
	allArtifacts := make([]db.ImageArtifact, 0)
	if err = s.Store.ListItems(&allArtifacts); err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	artifacts := make([]ArtifactInfo, 0)
	var totalSize int64
	for i := range allArtifacts {
		if allArtifacts[i].ImageID != image.ID {
			continue
		}
		artifacts = append(artifacts, ArtifactInfo{
			Path:   allArtifacts[i].Path,
			Digest: allArtifacts[i].Digest,
			Type:   allArtifacts[i].Type,
			Size:   allArtifacts[i].Size,
		})
		totalSize += allArtifacts[i].Size
	}

	info := &ImageInfo{
		Name: image.Name,
		Path: image.Path,
		Source: ImageSourceInfo{
			ID:       source.ID,
			Kind:     source.Kind,
			Location: source.Location,
		},
		Reference: version.Tag,
		Manifest: ManifestInfo{
			Digest:    version.ManifestDigest,
			MediaType: version.ManifestMediaType,
		},
		Artifacts: artifacts,
		TotalSize: totalSize,
		CreatedAt: image.CreatedAt,
		UpdatedAt: image.UpdatedAt,
	}

	return info, nil
}

// Remove deletes a downloaded image identified by its display name.
// It removes the artifact files from disk and all associated database records.
func (s *Service) Remove(imageName string) error {
	if err := s.ensureDefaults(); err != nil {
		return err
	}

	// Resolve the image name to the version ID via the name-refs bucket.
	versionID, err := s.Store.GetNameRef(imageName)
	if err != nil {
		return fmt.Errorf("failed to resolve image name %q: %w", imageName, err)
	}

	// Load the image record.
	var image db.Image
	if err = s.Store.GetItem(versionID, &image); err != nil {
		return fmt.Errorf("failed to get image record: %w", err)
	}

	// Load the version record.
	var version db.ImageVersion
	if err = s.Store.GetItem(image.VersionID, &version); err != nil {
		return fmt.Errorf("failed to get image version: %w", err)
	}

	// Load all artifacts for this image.
	allArtifacts := make([]db.ImageArtifact, 0)
	if err = s.Store.ListItems(&allArtifacts); err != nil {
		return fmt.Errorf("failed to list artifacts: %w", err)
	}

	artifacts := make([]db.ImageArtifact, 0)
	for i := range allArtifacts {
		if allArtifacts[i].ImageID == image.ID {
			artifacts = append(artifacts, allArtifacts[i])
		}
	}

	logger := s.Logger
	logger.Info("Removing image",
		"name", image.Name,
		"path", image.Path,
		"artifacts", len(artifacts),
	)

	// Remove artifact files from disk.
	if image.Path != "" {
		if err = s.FS.RemoveAll(image.Path); err != nil {
			logger.Warn("Failed to remove image directory", "path", image.Path, "error", err)
		}
	}

	// Delete artifact records.
	for i := range artifacts {
		if err = s.Store.DeleteItem(&artifacts[i]); err != nil {
			return fmt.Errorf("failed to delete artifact %q: %w", artifacts[i].ID, err)
		}
	}

	// Delete the image record.
	if err = s.Store.DeleteItem(&image); err != nil {
		return fmt.Errorf("failed to delete image record: %w", err)
	}

	// Delete the version record.
	if err = s.Store.DeleteItem(&version); err != nil {
		return fmt.Errorf("failed to delete version record: %w", err)
	}

	// Delete the name reference.
	if err = s.Store.RemoveNameRef(imageName); err != nil {
		return fmt.Errorf("failed to delete name reference: %w", err)
	}

	logger.Info("Image removed", "name", imageName)

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

	imageType, layers, err := inferImageTypeAndLayers(&manifest)
	if err != nil {
		return nil, err
	}

	return &InspectResult{
		Repository:       repository,
		Reference:        reference,
		Descriptor:       desc,
		Manifest:         manifest,
		ImageType:        imageType,
		Layers:           layers,
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
	if err := s.ensureDefaults(); err != nil {
		return "", err
	}

	c := s.Config
	fs := s.FS

	logger := s.Logger.With("imageRef", req.ImageRef)
	logger.Info("Getting image information")
	inspectResult, err := s.Inspect(ctx, req.ImageRef)
	if err != nil {
		return "", err
	}

	outputDir := fs.JoinPath(c.Images, imageDirectoryName(inspectResult))
	logger = logger.With("outputDir", outputDir)

	alreadyDownloaded, err := s.hasMatchingSavedManifest(inspectResult)
	if err != nil {
		return "", err
	}
	if alreadyDownloaded {
		logger.Info("Image artifacts already exist locally, skipping download")
		return outputDir, nil
	}

	if err = fs.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	imageID, err := persistImageMetadata(s.Store, inspectResult, outputDir)
	if err != nil {
		return "", err
	}

	repo, err := s.NewRepository(inspectResult.Repository)
	if err != nil {
		return "", fmt.Errorf("failed to create repository client: %w", err)
	}

	for _, layer := range inspectResult.Layers {
		err = layer.download(ctx, repo, fs, outputDir, os.Stdout)
		if err != nil {
			return "", fmt.Errorf("failed to download layer %s: %w", layer.Descriptor.Digest.String(), err)
		}
		if persistErr := persistImageArtifact(s.Store, imageID, outputDir, layer); persistErr != nil {
			return "", persistErr
		}
	}

	inspectJSON, err := json.MarshalIndent(inspectResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal inspect result: %w", err)
	}
	inspectPath := fs.JoinPath(outputDir, manifestFilename)
	if err = fs.WriteFile(inspectPath, inspectJSON, 0o644); err != nil {
		return "", fmt.Errorf("failed to write inspect result file: %w", err)
	}

	return outputDir, nil
}

func (s *Service) hasMatchingSavedManifest(inspectResult *InspectResult) (bool, error) {
	versionID := imageVersionID(inspectResult.Repository, inspectResult.Reference)
	var existingImage db.Image
	err := s.Store.GetItem(versionID, &existingImage)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return false, fmt.Errorf("failed to check for existing image version: %w", err)
		}
		return false, nil // No existing version, so no matching manifest.
	}
	// Get the corresponding image version
	var existingVersion db.ImageVersion
	err = s.Store.GetItem(existingImage.VersionID, &existingVersion)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return false, fmt.Errorf("failed to check for existing image version: %w", err)
		}
		return false, nil // No existing version, so no matching manifest.
	}
	if existingVersion.ManifestDigest != inspectResult.Descriptor.Digest.String() {
		return false, nil // Manifest digest doesn't match, so no matching manifest.
	}
	return true, nil // Found existing version with matching manifest digest, so we have a matching manifest.
}

func inferImageTypeAndLayers(manifest *specv1.Manifest) (ImageType, []PullLayer, error) {
	if manifest.ArtifactType == "" && len(manifest.Layers) == 1 {
		layerType := manifest.Layers[0].MediaType
		if layerType == rootfsMediaTypeDocker || layerType == rootfsMediaTypeOCI {
			return ImageTypeRootFS,
				[]PullLayer{{Descriptor: &manifest.Layers[0], FileName: ImageTypeRootFS.ImageFilename()}}, nil
		}
		return ImageTypeUnknown, nil, fmt.Errorf("unsupported rootfs layer media type: %s", layerType)
	}

	if manifest.ArtifactType == vhdxArtifactType {
		if len(manifest.Layers) != 1 {
			return ImageTypeUnknown, nil, fmt.Errorf("vhdx image must have exactly one layer")
		}
		if manifest.Layers[0].MediaType != vhdxMediaType {
			return ImageTypeUnknown, nil, fmt.Errorf("vhdx image layer media type must be %s", vhdxMediaType)
		}
		return ImageTypeVHDX,
			[]PullLayer{{Descriptor: &manifest.Layers[0], FileName: ImageTypeVHDX.ImageFilename()}}, nil
	}

	if manifest.ArtifactType == qcow2ArtifactType {
		if len(manifest.Layers) != 2 {
			return ImageTypeUnknown, nil, fmt.Errorf("qcow2 image must have exactly two layers")
		}
		layers := make([]PullLayer, 0, 2)
		hasQCOW2 := false
		hasIncusMetadata := false
		for i := range manifest.Layers {
			switch manifest.Layers[i].MediaType {
			case qcow2MediaType:
				hasQCOW2 = true
				layers = append(layers,
					PullLayer{Descriptor: &manifest.Layers[i], FileName: ImageTypeQCOW2.ImageFilename()})
			case incusMetadataType:
				hasIncusMetadata = true
				layers = append(layers, PullLayer{Descriptor: &manifest.Layers[i], FileName: "incus-metadata.bin"})
			}
		}
		if !hasQCOW2 || !hasIncusMetadata {
			return ImageTypeUnknown, nil, fmt.Errorf("qcow2 image must contain qcow2 and incus metadata layers")
		}
		return ImageTypeQCOW2, layers, nil
	}

	return ImageTypeUnknown, nil, fmt.Errorf("unsupported manifest artifactType %q", manifest.ArtifactType)
}

func (layer *PullLayer) download(
	ctx context.Context,
	repo Repository,
	fs host.FileEnvironment,
	outputDir string,
	out io.Writer,
) error {
	outputPath := fs.JoinPath(outputDir, layer.FileName)
	exists, err := fs.Exists(outputPath)
	if err != nil {
		return fmt.Errorf("failed to check if output file exists: %w", err)
	}
	if exists {
		return fmt.Errorf("output file %s already exists", outputPath)
	}
	destination, err := fs.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	readErr := readBlob(ctx, repo, layer, destination, out)
	if readErr != nil {
		if closeErr := destination.Close(); closeErr != nil {
			return fmt.Errorf("failed to close destination file after read error: %w", closeErr)
		}
		if removeErr := fs.Remove(outputPath); removeErr != nil {
			return fmt.Errorf("failed to remove incomplete output file after read error: %w", removeErr)
		}
		return readErr
	}
	return nil
}

func readBlob(ctx context.Context, repo Repository, layer *PullLayer, destination, out io.Writer) error {
	blobReader, err := repo.Fetch(ctx, *layer.Descriptor)
	if err != nil {
		return fmt.Errorf("failed to fetch blob %s: %w", layer.Descriptor.Digest.String(), err)
	}

	progress := NewProgressReader(blobReader, layer.Descriptor.Size, layer.FileName, out)

	_, err = io.Copy(destination, progress)
	if err != nil {
		if closeErr := blobReader.Close(); closeErr != nil {
			return fmt.Errorf("failed to close blob reader after read error: %w", closeErr)
		}
		return fmt.Errorf("failed to read blob data: %w", err)
	}
	if closeErr := blobReader.Close(); closeErr != nil {
		return fmt.Errorf("failed to close blob reader: %w", closeErr)
	}

	return nil
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
