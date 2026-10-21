package iknitectl_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	iknitectl "github.com/kaweezle/iknite/pkg/cmd/iknitectl"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/iknitectl/image"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestCreateImageCmdIncludesList(t *testing.T) {
	t.Parallel()

	cmd := iknitectl.CreateImageCmd(newImageScope(t))
	found, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "list", found.Name())
	require.Contains(t, found.Aliases, "ls")
}

func TestImageListCommandRendersTable(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	scope := newImageScope(t)
	req.NoError(scope.Invoke(seedImageListData))

	command := iknitectl.CreateImageListCmd(scope)
	require.NoError(t, command.ExecuteContext(t.Context()))

	output := testutil.Resolve[*bytes.Buffer](t, scope).String()
	require.Contains(t, output, "SOURCE")
	require.NotContains(t, output, " ID ")
	require.Contains(t, output, "ghcr.io/kaweezle/iknite")
	require.Contains(t, output, "2 [rootfs, rootfs]")
	require.Contains(t, output, "300B")
}

func TestImageListCommandShowsEmptyState(t *testing.T) {
	t.Parallel()

	s := newImageScope(t)
	command := iknitectl.CreateImageListCmd(s)
	require.NoError(t, command.ExecuteContext(t.Context()))
	output := testutil.Resolve[*bytes.Buffer](t, s).String()
	require.Contains(t, output, "No images found")
}

func newImageScope(t *testing.T) *dig.Scope {
	t.Helper()
	req := require.New(t)
	c := testutil.TestContainer(t)
	req.NoError(c.Provide(config.NewConfigOptions))
	req.NoError(c.Decorate(func(opts *config.ConfigOptions) *config.ConfigOptions {
		opts.ConfigDir = t.TempDir()
		return opts
	}))
	req.NoError(c.Provide(func(h host.Host, opts *config.ConfigOptions) (*config.Config, error) {
		c := &config.Config{}
		err := opts.Resolve(h, c)
		if err != nil {
			return nil, fmt.Errorf("resolving config: %w", err)
		}
		return c, nil
	}))
	req.NoError(c.Provide(func(c *config.Config) (*db.Store, error) {
		store, err := db.Open(c.Database)
		if err != nil {
			return nil, fmt.Errorf("opening store: %w", err)
		}
		t.Cleanup(func() {
			require.NoError(t, store.Close())
		})
		return store, nil
	}))
	req.NoError(c.Provide(func(store *db.Store) image.MetadataStore { return store }))

	req.NoError(c.Provide(image.NewService))

	return c.Scope("image-list")
}

func seedImageListData(t *testing.T, c *config.Config) {
	t.Helper()

	store, err := db.Open(c.Database)
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
		Name:      "ghcr.io/kaweezle/iknite:latest",
		Path:      "/tmp/images/rootfs",
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
