// cSpell: words testpackage specv qcow2 VMQCOW2 VMVHDX
package image

import (
	"encoding/json"
	"path/filepath"
	"testing"

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

	source, err := store.GetImageSource("ghcr.io/kaweezle/iknite-vm-qcow2")
	require.NoError(t, err)
	require.Equal(t, "registry", source.Kind)
	require.Equal(t, inspectResult.Repository, source.Location)

	version, err := store.GetImageVersion(imageID)
	require.NoError(t, err)
	require.Equal(t, source.ID, version.SourceID)
	require.Equal(t, inspectResult.Reference, version.Tag)
	require.Equal(t, inspectResult.Descriptor.Digest.String(), version.ManifestDigest)

	imageRecord, err := store.GetImage(imageID)
	require.NoError(t, err)
	require.Equal(t, version.ID, imageRecord.VersionID)
	require.Equal(t, outputDir, imageRecord.Name)
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

	existing, err := store.GetImageSource(sourceID)
	require.NoError(t, err)
	existing.Location = "stale"
	require.NoError(t, store.UpdateImageSource(existing))

	_, err = persistImageSource(store, inspectResult)
	require.NoError(t, err)

	updated, err := store.GetImageSource(sourceID)
	require.NoError(t, err)
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

	version, err := store.GetImageVersion(versionID)
	require.NoError(t, err)
	version.Tag = "stale"
	require.NoError(t, store.UpdateImageVersion(version))

	_, err = persistImageVersion(store, inspectResult, sourceID)
	require.NoError(t, err)

	updated, err := store.GetImageVersion(versionID)
	require.NoError(t, err)
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
	artifact, err := store.GetImageArtifact(artifactID)
	require.NoError(t, err)
	require.Equal(t, imageID, artifact.ImageID)
	require.Equal(t, filepath.Join(outputDir, layer.FileName), artifact.Path)
	require.Equal(t, manifest.Layers[0].Digest.String(), artifact.Digest)
	require.Equal(t, db.ArtifactTypeQCOW2Image, artifact.Type)
	require.Equal(t, manifest.Layers[0].Size, artifact.Size)

	artifact.Path = "stale"
	require.NoError(t, store.UpdateImageArtifact(artifact))
	require.NoError(t, persistImageArtifact(store, imageID, outputDir, layer))

	updated, err := store.GetImageArtifact(artifactID)
	require.NoError(t, err)
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
