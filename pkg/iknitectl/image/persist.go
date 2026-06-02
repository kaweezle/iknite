package image

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/kaweezle/iknite/pkg/iknitectl/db"
)

func imageVersionID(repository, reference string) string {
	return fmt.Sprintf("%s@%s", repository, reference)
}

func imageArtifactID(imageID, digest string) string {
	return fmt.Sprintf("%s@%s", imageID, digest)
}

func artifactTypeFromMediaType(mediaType string) db.ArtifactType {
	switch mediaType {
	case rootfsMediaTypeDocker, rootfsMediaTypeOCI:
		return db.ArtifactTypeRootFS
	case vhdxMediaType:
		return db.ArtifactTypeVHDXImage
	case qcow2MediaType:
		return db.ArtifactTypeQCOW2Image
	case incusMetadataType:
		return db.ArtifactTypeIncusMetadata
	default:
		return db.ArtifactTypeUnknown
	}
}

func persistImageSource(store MetadataStore, inspectResult *InspectResult) (string, error) {
	sourceID := inspectResult.Repository
	source := &db.ImageSource{
		BaseModel: db.BaseModel{ID: sourceID},
		Kind:      "registry",
		Location:  inspectResult.Repository,
	}
	if err := store.CreateOrUpdateItem(source); err != nil {
		return "", fmt.Errorf("failed to create or update image source: %w", err)
	}

	return sourceID, nil
}

func persistImageVersion(store MetadataStore, inspectResult *InspectResult, sourceID string) (string, error) {
	manifestBytes, err := json.Marshal(inspectResult.Manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest for metadata store: %w", err)
	}

	versionID := imageVersionID(inspectResult.Repository, inspectResult.Reference)
	version := &db.ImageVersion{
		BaseModel:         db.BaseModel{ID: versionID},
		SourceID:          sourceID,
		Tag:               inspectResult.Reference,
		ManifestDigest:    inspectResult.Descriptor.Digest.String(),
		ManifestMediaType: inspectResult.Descriptor.MediaType,
		Manifest:          manifestBytes,
	}
	if err = store.CreateOrUpdateItem(version); err != nil {
		return "", fmt.Errorf("failed to create or update image version: %w", err)
	}

	return versionID, nil
}

func persistImageRecord(store MetadataStore, versionID, imageRef, outputDir string) (string, error) {
	imageRecord := &db.Image{
		BaseModel: db.BaseModel{ID: versionID},
		VersionID: versionID,
		Name:      imageRef,
		Path:      outputDir,
	}
	if err := store.CreateOrUpdateItem(imageRecord); err != nil {
		return "", fmt.Errorf("failed to create or update image record: %w", err)
	}

	return versionID, nil
}

func persistImageArtifact(store MetadataStore, imageID, outputDir string, layer pullLayer) error {
	artifact := &db.ImageArtifact{
		BaseModel:  db.BaseModel{ID: imageArtifactID(imageID, layer.Descriptor.Digest.String())},
		Descriptor: *layer.Descriptor,
		ImageID:    imageID,
		Path:       filepath.Join(outputDir, layer.FileName),
		Digest:     layer.Descriptor.Digest.String(),
		Type:       artifactTypeFromMediaType(layer.Descriptor.MediaType),
		Size:       layer.Descriptor.Size,
	}

	if err := store.CreateOrUpdateItem(artifact); err != nil {
		return fmt.Errorf("failed to create or update image artifact: %w", err)
	}

	return nil
}

func persistImageMetadata(store MetadataStore, inspectResult *InspectResult, outputDir string) (string, error) {
	sourceID, err := persistImageSource(store, inspectResult)
	if err != nil {
		return "", err
	}

	versionID, err := persistImageVersion(store, inspectResult, sourceID)
	if err != nil {
		return "", err
	}

	imageID, err := persistImageRecord(
		store,
		versionID,
		fmt.Sprintf("%s:%s", filepath.Base(inspectResult.Repository), inspectResult.Reference),
		outputDir,
	)
	if err != nil {
		return "", err
	}

	return imageID, nil
}
