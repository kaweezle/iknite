// cSpell: words testpackage specv qcow2 VMQCOW2 VHDX VMVHDX
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	specv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

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
	require.Equal(t, ArtifactRootFS, inspectResult.ImageType)
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
	require.Equal(t, ArtifactVMVHDX, inspectResult.ImageType)
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
	require.Equal(t, ArtifactVMQCOW2, inspectResult.ImageType)
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

	outputDir, err := svc.Pull(context.Background(), &PullRequest{ImageRef: "ghcr.io/kaweezle/iknite-vm-qcow2:latest"})
	require.NoError(t, err)
	require.Equal(t, "/home/alpine/.config/iknite/images/ghcr.io_kaweezle_iknite-vm-qcow2_latest", outputDir)

	qcowData, err := svc.FS.ReadFile(filepath.Join(outputDir, "disk.qcow2"))
	require.NoError(t, err)
	require.Equal(t, []byte("qcow-data"), qcowData)

	metaData, err := svc.FS.ReadFile(filepath.Join(outputDir, "incus-metadata.bin"))
	require.NoError(t, err)
	require.Equal(t, []byte("meta-data"), metaData)

	inspectRaw, err := svc.FS.ReadFile(filepath.Join(outputDir, inspectResultFileName))
	require.NoError(t, err)

	var inspectResult InspectResult
	require.NoError(t, json.Unmarshal(inspectRaw, &inspectResult))
	require.Equal(t, ArtifactVMQCOW2, inspectResult.ImageType)
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

	inspectRaw, err := service.FS.ReadFile(filepath.Join(outputDir, inspectResultFileName))
	require.NoError(t, err)

	var inspectResult InspectResult
	require.NoError(t, json.Unmarshal(inspectRaw, &inspectResult))
	require.Equal(t, ArtifactVMQCOW2, inspectResult.ImageType)
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
	return &Service{
		FS: fs,
		NewRepository: func(string) (Repository, error) {
			return fakeRepo, nil
		},
	}, fakeRepo
}
