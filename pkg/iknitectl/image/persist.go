package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/kaweezle/iknite/pkg/iknitectl/db"
)

func imageSourceID(repository string) string {
	return repository
}

func imageVersionID(repository, reference string) string {
	return fmt.Sprintf("%s@%s", repository, reference)
}

func imageRecordID(versionID string) string {
	return versionID
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
	sourceID := imageSourceID(inspectResult.Repository)
	source, getSourceErr := store.GetImageSource(sourceID)
	if getSourceErr != nil {
		if !errors.Is(getSourceErr, db.ErrNotFound) {
			return "", fmt.Errorf("failed to get image source: %w", getSourceErr)
		}
		source = &db.ImageSource{BaseModel: db.BaseModel{ID: sourceID}}
	}
	source.Kind = "registry"
	source.Location = inspectResult.Repository
	if errors.Is(getSourceErr, db.ErrNotFound) {
		if createErr := store.CreateImageSource(source); createErr != nil {
			return "", fmt.Errorf("failed to create image source: %w", createErr)
		}
	} else {
		if updateErr := store.UpdateImageSource(source); updateErr != nil {
			return "", fmt.Errorf("failed to update image source: %w", updateErr)
		}
	}

	return sourceID, nil
}

func persistImageVersion(store MetadataStore, inspectResult *InspectResult, sourceID string) (string, error) {
	manifestBytes, err := json.Marshal(inspectResult.Manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest for metadata store: %w", err)
	}

	versionID := imageVersionID(inspectResult.Repository, inspectResult.Reference)
	version, getVersionErr := store.GetImageVersion(versionID)
	if getVersionErr != nil {
		if !errors.Is(getVersionErr, db.ErrNotFound) {
			return "", fmt.Errorf("failed to get image version: %w", getVersionErr)
		}
		version = &db.ImageVersion{BaseModel: db.BaseModel{ID: versionID}}
	}
	version.SourceID = sourceID
	version.Tag = inspectResult.Reference
	version.ManifestDigest = inspectResult.Descriptor.Digest.String()
	version.ManifestMediaType = inspectResult.Descriptor.MediaType
	version.Manifest = manifestBytes
	if errors.Is(getVersionErr, db.ErrNotFound) {
		if createErr := store.CreateImageVersion(version); createErr != nil {
			return "", fmt.Errorf("failed to create image version: %w", createErr)
		}
	} else {
		if updateErr := store.UpdateImageVersion(version); updateErr != nil {
			return "", fmt.Errorf("failed to update image version: %w", updateErr)
		}
	}

	return versionID, nil
}

func persistImageRecord(store MetadataStore, versionID, outputDir string) (string, error) {
	imageID := imageRecordID(versionID)
	imageRecord, getImageErr := store.GetImage(imageID)
	if getImageErr != nil {
		if !errors.Is(getImageErr, db.ErrNotFound) {
			return "", fmt.Errorf("failed to get image record: %w", getImageErr)
		}
		imageRecord = &db.Image{BaseModel: db.BaseModel{ID: imageID}}
	}
	imageRecord.VersionID = versionID
	imageRecord.Name = outputDir
	if errors.Is(getImageErr, db.ErrNotFound) {
		if createErr := store.CreateImage(imageRecord); createErr != nil {
			return "", fmt.Errorf("failed to create image record: %w", createErr)
		}
	} else {
		if updateErr := store.UpdateImage(imageRecord); updateErr != nil {
			return "", fmt.Errorf("failed to update image record: %w", updateErr)
		}
	}

	return imageID, nil
}

func persistImageArtifact(store MetadataStore, imageID, outputDir string, layer pullLayer) error {
	artifactID := imageArtifactID(imageID, layer.Descriptor.Digest.String())
	artifact, getArtifactErr := store.GetImageArtifact(artifactID)
	if getArtifactErr != nil {
		if !errors.Is(getArtifactErr, db.ErrNotFound) {
			return fmt.Errorf("failed to get image artifact: %w", getArtifactErr)
		}
		artifact = &db.ImageArtifact{BaseModel: db.BaseModel{ID: artifactID}}
	}

	artifact.ImageID = imageID
	artifact.Path = filepath.Join(outputDir, layer.FileName)
	artifact.Digest = layer.Descriptor.Digest.String()
	artifact.Type = artifactTypeFromMediaType(layer.Descriptor.MediaType)
	artifact.Size = layer.Descriptor.Size

	if errors.Is(getArtifactErr, db.ErrNotFound) {
		if err := store.CreateImageArtifact(artifact); err != nil {
			return fmt.Errorf("failed to create image artifact: %w", err)
		}
	} else {
		if err := store.UpdateImageArtifact(artifact); err != nil {
			return fmt.Errorf("failed to update image artifact: %w", err)
		}
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

	imageID, err := persistImageRecord(store, versionID, outputDir)
	if err != nil {
		return "", err
	}

	return imageID, nil
}
