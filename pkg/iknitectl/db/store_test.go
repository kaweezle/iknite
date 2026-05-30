// cSpell: words bimg
package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	db "github.com/kaweezle/iknite/pkg/iknitectl/db"
)

type seededGraph struct {
	sourceID       string
	versionID      string
	imageID        string
	artifactID     string
	backendImageID string
	clusterID      string
}

func newTestStore(t *testing.T) *db.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "iknite.db")
	store, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	return store
}

func seedDataGraph(t *testing.T, store *db.Store) seededGraph {
	t.Helper()

	graph := seededGraph{
		sourceID:       "src-1",
		versionID:      "ver-1",
		imageID:        "img-1",
		artifactID:     "art-1",
		backendImageID: "bimg-1",
		clusterID:      "cl-1",
	}

	require.NoError(
		t,
		store.CreateItem(&db.ImageSource{
			BaseModel: db.BaseModel{ID: graph.sourceID},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)
	require.NoError(
		t,
		store.CreateItem(&db.ImageVersion{
			BaseModel: db.BaseModel{ID: graph.versionID},
			SourceID:  graph.sourceID,
			Tag:       "v0.7.1-devel-1.36.1",
		}),
	)
	require.NoError(
		t,
		store.CreateItem(&db.Image{
			BaseModel: db.BaseModel{ID: graph.imageID},
			VersionID: graph.versionID,
			Name:      "iknite-rootfs",
		}),
	)
	require.NoError(
		t,
		store.CreateItem(&db.ImageArtifact{
			BaseModel: db.BaseModel{ID: graph.artifactID},
			ImageID:   graph.imageID,
			Type:      db.ArtifactTypeRootFS,
			Path:      "/tmp/rootfs.tar.gz",
		}),
	)
	require.NoError(
		t,
		store.CreateItem(&db.BackendImage{
			BaseModel:  db.BaseModel{ID: graph.backendImageID},
			Backend:    "incus",
			ImageID:    graph.imageID,
			ExternalID: "incus-image-01",
		}),
	)
	require.NoError(
		t,
		store.CreateItem(&db.Cluster{
			BaseModel:      db.BaseModel{ID: graph.clusterID},
			Name:           "dev",
			Backend:        "incus",
			ImageID:        graph.imageID,
			BackendImageID: graph.backendImageID,
		}),
	)

	return graph
}

func TestOpenCreatesStoreReadyForWrites(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	require.NoError(
		t,
		store.CreateItem(&db.ImageSource{
			BaseModel: db.BaseModel{ID: "src"},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)

	sources, err := db.ListItems[db.ImageSource](store)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	sources = make([]db.ImageSource, 0)
	require.NoError(t, store.ListItems(&sources))
	require.Len(t, sources, 1)
}

func TestStoreCRUDLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	graph := seedDataGraph(t, store)

	source := &db.ImageSource{}
	err := store.GetItem(graph.sourceID, source)
	require.NoError(t, err)
	require.Equal(t, "registry", source.Kind)
	require.False(t, source.CreatedAt.IsZero())
	require.False(t, source.UpdatedAt.IsZero())

	version := &db.ImageVersion{}
	err = store.GetItem(graph.versionID, version)
	require.NoError(t, err)
	require.Equal(t, "v0.7.1-devel-1.36.1", version.Tag)
	require.False(t, version.CreatedAt.IsZero())
	require.False(t, version.UpdatedAt.IsZero())

	img := &db.Image{}
	err = store.GetItem(graph.imageID, img)
	require.NoError(t, err)
	require.Equal(t, graph.versionID, img.VersionID)
	require.False(t, img.CreatedAt.IsZero())
	require.False(t, img.UpdatedAt.IsZero())
	previousCreatedAt := img.CreatedAt
	previousUpdatedAt := img.UpdatedAt
	time.Sleep(time.Millisecond)

	artifact := &db.ImageArtifact{}
	err = store.GetItem(graph.artifactID, artifact)
	require.NoError(t, err)
	require.Equal(t, db.ArtifactTypeRootFS, artifact.Type)

	backendImage := &db.BackendImage{}
	err = store.GetItem(graph.backendImageID, backendImage)
	require.NoError(t, err)
	require.Equal(t, "incus", backendImage.Backend)

	cluster := &db.Cluster{}
	err = store.GetItem(graph.clusterID, cluster)
	require.NoError(t, err)
	require.Equal(t, "dev", cluster.Name)

	img.Name = "iknite-rootfs-updated"
	require.NoError(t, store.UpdateItem(img))
	imgAfterUpdate := &db.Image{}
	err = store.GetItem(graph.imageID, imgAfterUpdate)
	require.NoError(t, err)
	require.Equal(t, previousCreatedAt, imgAfterUpdate.CreatedAt)
	require.True(t, imgAfterUpdate.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, "iknite-rootfs-updated", imgAfterUpdate.Name)

	cluster.Workspace = "/workspace/my-repo"
	require.NoError(t, store.UpdateItem(cluster))
	clusterAfterUpdate := &db.Cluster{}
	err = store.GetItem(graph.clusterID, clusterAfterUpdate)
	require.NoError(t, err)
	require.False(t, clusterAfterUpdate.CreatedAt.IsZero())
	require.False(t, clusterAfterUpdate.UpdatedAt.IsZero())

	images, err := db.ListItems[db.Image](store)
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, "iknite-rootfs-updated", images[0].Name)

	clusters, err := db.ListItems[db.Cluster](store)
	require.NoError(t, err)
	require.Len(t, clusters, 1)
	require.Equal(t, "/workspace/my-repo", clusters[0].Workspace)

	require.NoError(t, store.DeleteItem(clusterAfterUpdate))
	require.NoError(t, store.DeleteItem(backendImage))
	require.NoError(t, store.DeleteItem(artifact))
	require.NoError(t, store.DeleteItem(imgAfterUpdate))
	require.NoError(t, store.DeleteItem(version))
	require.NoError(t, store.DeleteItem(source))

	cluster = &db.Cluster{}
	err = store.GetItem(graph.clusterID, cluster)
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestStoreValidatesReferences(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.CreateItem(store, &db.ImageVersion{
		BaseModel: db.BaseModel{ID: "ver-missing"},
		SourceID:  "missing-source",
		Tag:       "v1",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = db.CreateItem(store, &db.Image{BaseModel: db.BaseModel{ID: "img-missing"}, VersionID: "missing-version"})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = db.CreateItem(store, &db.BackendImage{
		BaseModel: db.BaseModel{ID: "bimg-missing"},
		Backend:   "incus",
		ImageID:   "missing-image",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = db.CreateItem(store, &db.Cluster{
		BaseModel:      db.BaseModel{ID: "cl-missing"},
		Name:           "dev",
		Backend:        "wsl",
		ImageID:        "missing",
		BackendImageID: "missing",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestStoreDetectsAlreadyExistsAndInvalidIDs(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(
		t,
		db.CreateItem(store, &db.ImageSource{
			BaseModel: db.BaseModel{ID: "src-1"},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)
	err := db.CreateItem(store, &db.ImageSource{
		BaseModel: db.BaseModel{ID: "src-1"},
		Kind:      "registry",
		Location:  "ghcr.io/kaweezle/iknite",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrAlreadyExists)

	generated := &db.ImageSource{BaseModel: db.BaseModel{}, Kind: "registry"}
	err = store.CreateItem(generated)
	require.NoError(t, err)
	require.NotEmpty(t, generated.ID)
	parsed, err := uuid.Parse(generated.ID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())

	missing := &db.ImageSource{BaseModel: db.BaseModel{ID: "missing"}, Kind: "registry"}
	err = db.DeleteItem(store, missing)
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = db.DeleteItem(store, &db.ImageSource{BaseModel: db.BaseModel{ID: ""}, Kind: "registry"})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrInvalidID)
}

func TestStoreCreateOrUpdateItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	source := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "src-upsert"},
		Kind:      "registry",
		Location:  "ghcr.io/kaweezle/iknite",
	}
	require.NoError(t, store.CreateOrUpdateItem(source))

	stored := &db.ImageSource{}
	require.NoError(t, store.GetItem("src-upsert", stored))
	require.Equal(t, source.Location, stored.Location)
	require.False(t, stored.CreatedAt.IsZero())
	require.False(t, stored.UpdatedAt.IsZero())

	previousCreatedAt := stored.CreatedAt
	previousUpdatedAt := stored.UpdatedAt
	time.Sleep(time.Millisecond)

	updatedSource := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "src-upsert"},
		Kind:      "registry",
		Location:  "ghcr.io/kaweezle/iknite-new",
	}
	require.NoError(t, store.CreateOrUpdateItem(updatedSource))

	storedAfterUpdate := &db.ImageSource{}
	require.NoError(t, store.GetItem("src-upsert", storedAfterUpdate))
	require.Equal(t, previousCreatedAt, storedAfterUpdate.CreatedAt)
	require.True(t, storedAfterUpdate.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, "ghcr.io/kaweezle/iknite-new", storedAfterUpdate.Location)
}
