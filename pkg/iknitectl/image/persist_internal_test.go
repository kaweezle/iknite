// cSpell: words testpackage specv qcow2 VMQCOW2 VMVHDX
package image

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	specv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/iknitectl/db"
)

func TestPersistImageMetadataStoresSourceVersionAndImage(t *testing.T) {
	t.Parallel()

	store := newPersistenceStore(t)
	manifest := specv1.Manifest{
		ArtifactType: qcow2ArtifactType,
		Layers: []specv1.Descriptor{
			{MediaType: qcow2MediaType, Digest: digest.FromString("qcow")},
			{MediaType: incusMetadataType, Digest: digest.FromString("meta")},
		},
	}
	inspectResult := newInspectResult(
		"ghcr.io/kaweezle/iknite-vm-qcow2",
		&manifest,
	)
	outputDir := "/tmp/images/qcow2"

	imageID, err := persistImageMetadata(store, inspectResult, outputDir)
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/kaweezle/iknite-vm-qcow2@latest", imageID)

	source := &db.ImageSource{}
	err = store.GetItem("ghcr.io/kaweezle/iknite-vm-qcow2", source)
	require.NoError(t, err)
	require.Equal(t, "registry", source.Kind)
	require.Equal(t, inspectResult.Repository, source.Location)

	version := &db.ImageVersion{}
	err = store.GetItem(imageID, version)
	require.NoError(t, err)
	require.Equal(t, source.ID, version.SourceID)
	require.Equal(t, inspectResult.Reference, version.Tag)
	require.Equal(t, inspectResult.Descriptor.Digest.String(), version.ManifestDigest)

	imageRecord := &db.Image{}
	err = store.GetItem(imageID, imageRecord)
	require.NoError(t, err)
	require.Equal(t, version.ID, imageRecord.VersionID)
	require.Equal(t, fmt.Sprintf("%s:%s", filepath.Base(inspectResult.Repository), inspectResult.Reference),
		imageRecord.Name)
	require.Equal(t, outputDir, imageRecord.Path)
}

func TestPersistImageSourceUpdatesExistingSource(t *testing.T) {
	t.Parallel()

	store := newPersistenceStore(t)
	inspectResult := newInspectResult(
		"ghcr.io/kaweezle/iknite",
		&specv1.Manifest{
			Layers: []specv1.Descriptor{{
				MediaType: rootfsMediaTypeDocker,
				Digest:    digest.FromString("rootfs"),
			}},
		},
	)

	sourceID, err := persistImageSource(store, inspectResult)
	require.NoError(t, err)

	existing := &db.ImageSource{}
	err = store.GetItem(sourceID, existing)
	require.NoError(t, err)
	previousCreatedAt := existing.CreatedAt
	previousUpdatedAt := existing.UpdatedAt
	existing.Location = "stale"
	require.NoError(t, db.UpdateItem(store, existing))
	time.Sleep(time.Millisecond)

	_, err = persistImageSource(store, inspectResult)
	require.NoError(t, err)

	updated := &db.ImageSource{}
	err = store.GetItem(sourceID, updated)
	require.NoError(t, err)
	require.Equal(t, previousCreatedAt, updated.CreatedAt)
	require.True(t, updated.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, inspectResult.Repository, updated.Location)
}

func TestPersistImageVersionUpdatesExistingVersion(t *testing.T) {
	t.Parallel()

	store := newPersistenceStore(t)
	manifest := specv1.Manifest{
		Layers: []specv1.Descriptor{{MediaType: rootfsMediaTypeDocker, Digest: digest.FromString("rootfs")}},
	}
	inspectResult := newInspectResult("ghcr.io/kaweezle/iknite", &manifest)

	sourceID, err := persistImageSource(store, inspectResult)
	require.NoError(t, err)
	versionID, err := persistImageVersion(store, inspectResult, sourceID)
	require.NoError(t, err)

	version := &db.ImageVersion{}
	err = store.GetItem(versionID, version)
	require.NoError(t, err)
	previousCreatedAt := version.CreatedAt
	previousUpdatedAt := version.UpdatedAt
	version.Tag = "stale"
	require.NoError(t, db.UpdateItem(store, version))
	time.Sleep(time.Millisecond)

	_, err = persistImageVersion(store, inspectResult, sourceID)
	require.NoError(t, err)

	updated := &db.ImageVersion{}
	err = store.GetItem(versionID, updated)
	require.NoError(t, err)
	require.Equal(t, previousCreatedAt, updated.CreatedAt)
	require.True(t, updated.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, inspectResult.Reference, updated.Tag)
	require.Equal(t, inspectResult.Descriptor.Digest.String(), updated.ManifestDigest)
}

func TestPersistImageArtifactCreatesAndUpdatesArtifact(t *testing.T) {
	t.Parallel()

	store := newPersistenceStore(t)
	manifest := specv1.Manifest{
		ArtifactType: qcow2ArtifactType,
		Layers: []specv1.Descriptor{
			{MediaType: qcow2MediaType, Digest: digest.FromString("qcow"), Size: 123},
			{MediaType: incusMetadataType, Digest: digest.FromString("meta"), Size: 45},
		},
	}
	inspectResult := newInspectResult("ghcr.io/kaweezle/iknite-vm-qcow2", &manifest)
	outputDir := "/tmp/images/qcow2"

	imageID, err := persistImageMetadata(store, inspectResult, outputDir)
	require.NoError(t, err)

	layer := pullLayer{Descriptor: &manifest.Layers[0], FileName: "disk.qcow2"}
	require.NoError(t, persistImageArtifact(store, imageID, outputDir, layer))

	artifactID := imageID + "@" + manifest.Layers[0].Digest.String()

	artifact, err := db.GetItem[db.ImageArtifact](store, artifactID)
	require.NoError(t, err)
	require.Equal(t, imageID, artifact.ImageID)
	require.Equal(t, filepath.Join(outputDir, layer.FileName), artifact.Path)
	require.Equal(t, manifest.Layers[0].Digest.String(), artifact.Digest)
	require.Equal(t, db.ArtifactTypeQCOW2Image, artifact.Type)
	require.Equal(t, manifest.Layers[0].Size, artifact.Size)

	artifact.Path = "stale"
	require.NoError(t, db.UpdateItem(store, artifact))
	previousCreatedAt := artifact.CreatedAt
	previousUpdatedAt := artifact.UpdatedAt
	time.Sleep(time.Millisecond)
	require.NoError(t, persistImageArtifact(store, imageID, outputDir, layer))

	updated, err := db.GetItem[db.ImageArtifact](store, artifactID)
	require.NoError(t, err)
	require.Equal(t, previousCreatedAt, updated.CreatedAt)
	require.True(t, updated.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, filepath.Join(outputDir, layer.FileName), updated.Path)
}

func newPersistenceStore(t *testing.T) *db.Store {
	t.Helper()

	store, err := db.Open(filepath.Join(t.TempDir(), "iknite.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	return store
}

func newInspectResult(repository string, manifest *specv1.Manifest) *InspectResult {
	manifestDigest := digest.FromBytes(mustMarshalManifest(manifest))

	return &InspectResult{
		Repository: repository,
		Reference:  "latest",
		Descriptor: specv1.Descriptor{
			Digest:    manifestDigest,
			MediaType: "application/vnd.oci.image.manifest.v1+json",
		},
		Manifest:         *manifest,
		ManifestTypeHint: manifest.ArtifactType,
		ImageType:        mustInferImageType(manifest),
	}
}

func mustMarshalManifest(manifest *specv1.Manifest) []byte {
	payload, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}

	return payload
}

func mustInferImageType(manifest *specv1.Manifest) ImageType {
	imageType, err := inferImageType(manifest)
	if err != nil {
		panic(err)
	}

	return imageType
}
