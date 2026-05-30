package iknitectl_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	iknitectl "github.com/kaweezle/iknite/pkg/cmd/iknitectl"
	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestCreateImageCmdIncludesList(t *testing.T) {
	t.Parallel()

	baseService := newImageListBaseService(t)
	cmd := iknitectl.CreateImageCmd(baseService)
	found, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "list", found.Name())
	require.Contains(t, found.Aliases, "ls")
}

func TestImageListCommandRendersTable(t *testing.T) {
	t.Parallel()

	baseService := newImageListBaseService(t)
	seedImageListData(t, baseService.Config().Database)

	command := iknitectl.CreateImageListCmd(baseService)
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stdout)
	require.NoError(t, command.Execute())

	output := stdout.String()
	require.Contains(t, output, "SOURCE")
	require.NotContains(t, output, " ID ")
	require.Contains(t, output, "ghcr.io/kaweezle/iknite")
	require.Contains(t, output, "2 [rootfs, rootfs]")
	require.Contains(t, output, "300B")
}

func TestImageListCommandShowsEmptyState(t *testing.T) {
	t.Parallel()

	baseService := newImageListBaseService(t)
	command := iknitectl.CreateImageListCmd(baseService)
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stdout)
	require.NoError(t, command.Execute())
	require.Contains(t, stdout.String(), "No images found")
}

func newImageListBaseService(t *testing.T) base.ServiceInterface {
	t.Helper()

	h := testutil.NewDummyUserHost()
	logger := testutil.TestLogger(t)
	opts := config.NewConfigOptions(h)
	opts.ConfigDir = t.TempDir()
	return base.NewService(h, logger, opts)
}

func seedImageListData(t *testing.T, databasePath string) {
	t.Helper()

	store, err := db.Open(databasePath)
	require.NoError(t, err)

	require.NoError(t, db.CreateItem(store, &db.ImageSource{
		BaseModel: db.BaseModel{ID: "ghcr.io/kaweezle/iknite"},
		Kind:      "registry",
		Location:  "ghcr.io/kaweezle/iknite",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageVersion{
		BaseModel:         db.BaseModel{ID: "ghcr.io/kaweezle/iknite@latest"},
		SourceID:          "ghcr.io/kaweezle/iknite",
		Tag:               "latest",
		ManifestDigest:    "sha256:1",
		ManifestMediaType: "application/vnd.oci.image.manifest.v1+json",
	}))
	require.NoError(t, db.CreateItem(store, &db.Image{
		BaseModel: db.BaseModel{ID: "ghcr.io/kaweezle/iknite@latest"},
		VersionID: "ghcr.io/kaweezle/iknite@latest",
		Name:      "/tmp/images/rootfs",
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "ghcr.io/kaweezle/iknite@latest@sha256:one"},
		ImageID:   "ghcr.io/kaweezle/iknite@latest",
		Path:      "/tmp/images/rootfs/rootfs.tar.gz",
		Digest:    "sha256:one",
		Type:      db.ArtifactTypeRootFS,
		Size:      100,
	}))
	require.NoError(t, db.CreateItem(store, &db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "ghcr.io/kaweezle/iknite@latest@sha256:two"},
		ImageID:   "ghcr.io/kaweezle/iknite@latest",
		Path:      "/tmp/images/rootfs/incus-metadata.bin",
		Digest:    "sha256:two",
		Type:      db.ArtifactTypeRootFS,
		Size:      200,
	}))
	require.NoError(t, store.Close())
}
