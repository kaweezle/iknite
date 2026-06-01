// cSpell: words testpackage specv qcow2 VMQCOW2 VMVHDX
// cSpell: words imagemocks
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	specv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/testutil"
)

type fakeRepository struct {
	blobs       map[string][]byte
	descriptor  specv1.Descriptor
	manifest    []byte
	blobFetches int
}

func (f *fakeRepository) Resolve(_ context.Context, _ string) (specv1.Descriptor, error) {
	return f.descriptor, nil
}

//nolint:gocritic // Method signature must match production interface.
func (f *fakeRepository) Fetch(_ context.Context, target specv1.Descriptor) (io.ReadCloser, error) {
	if target.Digest == f.descriptor.Digest {
		return io.NopCloser(bytes.NewReader(f.manifest)), nil
	}
	f.blobFetches++
	return io.NopCloser(bytes.NewReader(f.blobs[target.Digest.String()])), nil
}

func TestSplitImageReference(t *testing.T) {
	t.Parallel()

	repo, ref := splitImageReference("ghcr.io/kaweezle/iknite")
	require.Equal(t, "ghcr.io/kaweezle/iknite", repo)
	require.Equal(t, "latest", ref)

	repo, ref = splitImageReference("ghcr.io/kaweezle/iknite:v1")
	require.Equal(t, "ghcr.io/kaweezle/iknite", repo)
	require.Equal(t, "v1", ref)

	repo, ref = splitImageReference("ghcr.io/kaweezle/iknite@sha256:abcd")
	require.Equal(t, "ghcr.io/kaweezle/iknite", repo)
	require.Equal(t, "sha256:abcd", ref)
}

func TestInspectInfersRootFS(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		Layers: []specv1.Descriptor{{
			MediaType: rootfsMediaTypeDocker,
			Digest:    digest.FromString("rootfs"),
		}},
	}

	svc := newServiceForManifest(t, &manifest, map[string][]byte{digest.FromString("rootfs").String(): []byte("r")})
	inspectResult, err := svc.Inspect(context.Background(), "ghcr.io/kaweezle/iknite:latest")
	require.NoError(t, err)
	require.Equal(t, ImageTypeRootFS, inspectResult.ImageType)
	require.Empty(t, inspectResult.ManifestTypeHint)
}

func TestInspectInfersVHDX(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		ArtifactType: vhdxArtifactType,
		Layers: []specv1.Descriptor{{
			MediaType: vhdxMediaType,
			Digest:    digest.FromString("vhdx"),
		}},
	}

	svc := newServiceForManifest(t, &manifest, map[string][]byte{digest.FromString("vhdx").String(): []byte("v")})
	inspectResult, err := svc.Inspect(context.Background(), "ghcr.io/kaweezle/iknite-vm-vhdx:latest")
	require.NoError(t, err)
	require.Equal(t, ImageTypeVHDX, inspectResult.ImageType)
	require.Equal(t, vhdxArtifactType, inspectResult.ManifestTypeHint)
}

func TestInspectInfersQCOW2(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		ArtifactType: qcow2ArtifactType,
		Layers: []specv1.Descriptor{
			{MediaType: qcow2MediaType, Digest: digest.FromString("qcow")},
			{MediaType: incusMetadataType, Digest: digest.FromString("meta")},
		},
	}

	blobs := map[string][]byte{
		digest.FromString("qcow").String(): []byte("q"),
		digest.FromString("meta").String(): []byte("m"),
	}
	svc := newServiceForManifest(t, &manifest, blobs)
	inspectResult, err := svc.Inspect(context.Background(), "ghcr.io/kaweezle/iknite-vm-qcow2:latest")
	require.NoError(t, err)
	require.Equal(t, ImageTypeQCOW2, inspectResult.ImageType)
	require.Equal(t, qcow2ArtifactType, inspectResult.ManifestTypeHint)
}

func TestPullStoresArtifactsAndInspectJSON(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		ArtifactType: qcow2ArtifactType,
		Layers: []specv1.Descriptor{
			{MediaType: qcow2MediaType, Digest: digest.FromString("qcow")},
			{MediaType: incusMetadataType, Digest: digest.FromString("meta")},
		},
	}

	blobs := map[string][]byte{
		digest.FromString("qcow").String(): []byte("qcow-data"),
		digest.FromString("meta").String(): []byte("meta-data"),
	}
	svc := newServiceForManifest(t, &manifest, blobs)
	expectedOutputDir := "/home/alpine/.config/iknite/images/ghcr.io_kaweezle_iknite-vm-qcow2_latest"

	outputDir, err := svc.Pull(context.Background(), &PullRequest{ImageRef: "ghcr.io/kaweezle/iknite-vm-qcow2:latest"})
	require.NoError(t, err)
	require.Equal(t, expectedOutputDir, outputDir)

	qcowData, err := svc.FS.ReadFile(filepath.Join(outputDir, "disk.qcow2"))
	require.NoError(t, err)
	require.Equal(t, []byte("qcow-data"), qcowData)

	metaData, err := svc.FS.ReadFile(filepath.Join(outputDir, "incus-metadata.bin"))
	require.NoError(t, err)
	require.Equal(t, []byte("meta-data"), metaData)

	inspectRaw, err := svc.FS.ReadFile(filepath.Join(outputDir, manifestFilename))
	require.NoError(t, err)

	var inspectResult InspectResult
	require.NoError(t, json.Unmarshal(inspectRaw, &inspectResult))
	require.Equal(t, ImageTypeQCOW2, inspectResult.ImageType)
	require.Equal(t, qcow2ArtifactType, inspectResult.ManifestTypeHint)
}

func TestPullSkipsDownloadWhenManifestMatches(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		ArtifactType: qcow2ArtifactType,
		Layers: []specv1.Descriptor{
			{MediaType: qcow2MediaType, Digest: digest.FromString("qcow")},
			{MediaType: incusMetadataType, Digest: digest.FromString("meta")},
		},
	}

	blobs := map[string][]byte{
		digest.FromString("qcow").String(): []byte("qcow-data"),
		digest.FromString("meta").String(): []byte("meta-data"),
	}

	service, repo := newServiceAndRepoForManifest(t, &manifest, blobs)

	_, err := service.Pull(
		context.Background(),
		&PullRequest{ImageRef: "ghcr.io/kaweezle/iknite-vm-qcow2:latest"},
	)
	require.NoError(t, err)
	require.Equal(t, 2, repo.blobFetches)

	outputDir, err := service.Pull(
		context.Background(),
		&PullRequest{ImageRef: "ghcr.io/kaweezle/iknite-vm-qcow2:latest"},
	)
	require.NoError(t, err)
	require.Equal(t, 2, repo.blobFetches)

	inspectRaw, err := service.FS.ReadFile(filepath.Join(outputDir, manifestFilename))
	require.NoError(t, err)

	var inspectResult InspectResult
	require.NoError(t, json.Unmarshal(inspectRaw, &inspectResult))
	require.Equal(t, ImageTypeQCOW2, inspectResult.ImageType)
}

func TestPullStoresRootFSAndInspectJSON(t *testing.T) {
	t.Parallel()

	manifest := specv1.Manifest{
		Layers: []specv1.Descriptor{{
			MediaType: rootfsMediaTypeDocker,
			Digest:    digest.FromString("rootfs"),
		}},
	}

	blobs := map[string][]byte{digest.FromString("rootfs").String(): []byte("rootfs-data")}
	service, _ := newServiceAndRepoForManifest(t, &manifest, blobs)
	outputDir, err := service.Pull(context.Background(), &PullRequest{ImageRef: "ghcr.io/kaweezle/iknite:latest"})
	require.NoError(t, err)

	rootfsData, err := service.FS.ReadFile(path.Join(outputDir, "rootfs.tar.gz"))
	require.NoError(t, err)
	require.Equal(t, []byte("rootfs-data"), rootfsData)

	inspectRaw, err := service.FS.ReadFile(path.Join(outputDir, manifestFilename))
	require.NoError(t, err)

	var inspectResult InspectResult
	require.NoError(t, json.Unmarshal(inspectRaw, &inspectResult))
	require.Equal(t, ImageTypeRootFS, inspectResult.ImageType)
}

func TestListImages(t *testing.T) {
	t.Parallel()

	store := newPersistenceStore(t)
	fs := testutil.NewDummyUserHost()
	configOptions := config.NewConfigOptions(fs)
	configOptions.ConfigDir = t.TempDir()
	c := &config.Config{}
	require.NoError(t, configOptions.Resolve(fs, c))

	require.NoError(t, db.CreateItem(store, &db.ImageSource{
		BaseModel: db.BaseModel{ID: "repo-a"},
		Kind:      "registry",
		Location:  "repo-a",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageVersion{
		BaseModel:         db.BaseModel{ID: "repo-a@v1"},
		SourceID:          "repo-a",
		Tag:               "v1",
		ManifestDigest:    "sha256:aaa",
		ManifestMediaType: "application/vnd.oci.image.manifest.v1+json",
	}))
	require.NoError(t, db.CreateItem(store, &db.Image{
		BaseModel: db.BaseModel{ID: "image-a"},
		VersionID: "repo-a@v1",
		Name:      "repo-a:v1",
		Path:      "/tmp/images/a",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "artifact-a-1"},
		ImageID:   "image-a",
		Type:      db.ArtifactTypeRootFS,
	}))

	require.NoError(t, db.CreateItem(store, &db.ImageSource{
		BaseModel: db.BaseModel{ID: "repo-b"},
		Kind:      "registry",
		Location:  "repo-b",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageVersion{
		BaseModel:         db.BaseModel{ID: "repo-b@v2"},
		SourceID:          "repo-b",
		Tag:               "v2",
		ManifestDigest:    "sha256:bbb",
		ManifestMediaType: "application/vnd.oci.image.manifest.v1+json",
	}))
	require.NoError(t, db.CreateItem(store, &db.Image{
		BaseModel: db.BaseModel{ID: "image-b"},
		VersionID: "repo-b@v2",
		Name:      "repo-b:v2",
		Path:      "/tmp/images/z",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "artifact-b-1"},
		ImageID:   "image-b",
		Type:      db.ArtifactTypeRootFS,
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "artifact-b-2"},
		ImageID:   "image-b",
		Type:      db.ArtifactTypeIncusMetadata,
	}))

	svc := &Service{FS: fs, Logger: testutil.TestLogger(t), Config: c, Store: store}
	items, err := svc.ListImages()
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "/tmp/images/a", items[0].Path)
	require.Equal(t, "repo-a", items[0].Source)
	require.Equal(t, "v1", items[0].Reference)
	require.Equal(t, "1 [rootfs]", items[0].Artifacts)
	require.EqualValues(t, 0, items[0].TotalSize)
	require.Equal(t, "/tmp/images/z", items[1].Path)
	require.Equal(t, "repo-b", items[1].Source)
	require.Equal(t, "v2", items[1].Reference)
	require.Equal(t, "2 [incus-metadata, rootfs]", items[1].Artifacts)
	require.EqualValues(t, 0, items[1].TotalSize)
}

func newServiceForManifest(t *testing.T, manifest *specv1.Manifest, blobs map[string][]byte) *Service {
	t.Helper()

	service, _ := newServiceAndRepoForManifest(t, manifest, blobs)

	return service
}

func newServiceAndRepoForManifest(
	t *testing.T,
	manifest *specv1.Manifest,
	blobs map[string][]byte,
) (*Service, *fakeRepository) {
	t.Helper()

	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)

	manifestDigest := digest.FromBytes(manifestBytes)
	fakeRepo := &fakeRepository{
		descriptor: specv1.Descriptor{Digest: manifestDigest},
		manifest:   manifestBytes,
		blobs:      blobs,
	}

	fs := testutil.NewDummyUserHost()
	c := &config.Config{}
	require.NoError(t, config.NewConfigOptions(fs).Resolve(fs, c))
	store, err := db.Open(filepath.Join(t.TempDir(), "iknite.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	return &Service{
		FS: fs,
		NewRepository: func(string) (Repository, error) {
			return fakeRepo, nil
		},
		Logger: testutil.TestLogger(t),
		Config: c,
		Store:  store,
	}, fakeRepo
}
