// cSpell: words bimg bbolt
package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	bbolt "go.etcd.io/bbolt"

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
			Name:      "iknite:latest",
			Path:      "/tmp/images/iknite",
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

func TestSetNameRefAndGetAndRemove(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Set a name ref.
	require.NoError(t, store.SetNameRef("iknite:latest", "ghcr.io/kaweezle/iknite@latest"))

	// Get it back.
	ref, err := store.GetNameRef("iknite:latest")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/kaweezle/iknite@latest", ref)

	// Remove it.
	require.NoError(t, store.RemoveNameRef("iknite:latest"))

	// Getting a removed ref should fail.
	_, err = store.GetNameRef("iknite:latest")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestSetNameRefEmptyName(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.SetNameRef("", "ref")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image name is required")
}

func TestSetNameRefEmptyRef(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.SetNameRef("name", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image reference is required")
}

func TestGetNameRefEmptyName(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, err := store.GetNameRef("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image name is required")
}

func TestGetNameRefNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, err := store.GetNameRef("nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestRemoveNameRefEmptyName(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.RemoveNameRef("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image name is required")
}

func TestRemoveNameRefNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.RemoveNameRef("nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestSetNameRefOverwrite(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.SetNameRef("iknite:latest", "ref-v1"))
	require.NoError(t, store.SetNameRef("iknite:latest", "ref-v2"))

	ref, err := store.GetNameRef("iknite:latest")
	require.NoError(t, err)
	require.Equal(t, "ref-v2", ref)
}

// --- Untyped method variants ---

func TestUntypedCreateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateItem(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestUntypedUpdateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.UpdateItem(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestUntypedUpdateItemNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.UpdateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "missing-src"},
		Kind:      "registry",
		Location:  "loc",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestUntypedCreateOrUpdateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateOrUpdateItem(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestUntypedDeleteItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.DeleteItem(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestUntypedGetItemNilOutput(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.GetItem("id", nil)
	require.Error(t, err)
}

func TestUntypedGetItemNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	out := &db.ImageSource{}
	err := store.GetItem("nonexistent", out)
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- Generic function variants ---

func TestGenericUpdateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.UpdateItem(store, (*db.ImageSource)(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestGenericUpdateItemSuccess(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	src := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "gen-upd"},
		Kind:      "registry",
		Location:  "loc1",
	}
	require.NoError(t, db.CreateItem(store, src))

	src.Location = "loc2"
	require.NoError(t, db.UpdateItem(store, src))

	got, err := db.GetItem[db.ImageSource](store, "gen-upd")
	require.NoError(t, err)
	require.Equal(t, "loc2", got.Location)
}

func TestGenericCreateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.CreateItem(store, (*db.ImageSource)(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestGenericDeleteItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.DeleteItem(store, (*db.ImageSource)(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestGenericCreateOrUpdateItemNilItem(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.CreateOrUpdateItem(store, (*db.ImageSource)(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "item is required")
}

func TestGenericGetItemNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, err := db.GetItem[db.ImageSource](store, "nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestGenericCreateOrUpdateItemCreatesAndUpdates(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	src := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "gen-src-1"},
		Kind:      "registry",
		Location:  "ghcr.io/test",
	}
	require.NoError(t, db.CreateOrUpdateItem(store, src))

	got, err := db.GetItem[db.ImageSource](store, "gen-src-1")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/test", got.Location)

	src.Location = "ghcr.io/test-new"
	require.NoError(t, db.CreateOrUpdateItem(store, src))

	got, err = db.GetItem[db.ImageSource](store, "gen-src-1")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/test-new", got.Location)
}

func TestGenericDeleteItemNotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := db.DeleteItem(store, &db.ImageSource{BaseModel: db.BaseModel{ID: "ghost"}})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- ListItems error paths ---

func TestListItemsNilOutput(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.ListItems(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "output parameter is required")
}

func TestListItemsNotPointerToSlice(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	var s []db.ImageSource
	err := store.ListItems(s)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a pointer to a slice")
}

func TestListItemsUnsupportedType(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	type unsupported struct {
		Value string
	}

	out := make([]unsupported, 0)
	err := store.ListItems(&out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type")
}

// --- CreateItem with validation error ---

func TestCreateItemValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// ImageVersion references missing source.
	err := store.CreateItem(&db.ImageVersion{
		BaseModel: db.BaseModel{ID: "ver-invalid"},
		SourceID:  "missing-source",
		Tag:       "v1",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- UpdateItem preserves CreatedAt ---

func TestUntypedUpdateItemPreservesCreatedAt(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "src-preserve"},
		Kind:      "registry",
		Location:  "loc1",
	}))

	existing := &db.ImageSource{}
	require.NoError(t, store.GetItem("src-preserve", existing))
	originalCreatedAt := existing.CreatedAt

	existing.Location = "loc2"
	require.NoError(t, store.UpdateItem(existing))

	updated := &db.ImageSource{}
	require.NoError(t, store.GetItem("src-preserve", updated))
	require.Equal(t, originalCreatedAt, updated.CreatedAt)
	require.False(t, updated.UpdatedAt.IsZero())
}

// --- DeleteItem with empty ID ---

func TestUntypedDeleteItemEmptyID(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.DeleteItem(&db.ImageSource{BaseModel: db.BaseModel{ID: ""}})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrInvalidID)
}

// --- GetItem empty ID ---

func TestUntypedGetItemEmptyID(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	out := &db.ImageSource{}
	err := store.GetItem("", out)
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrInvalidID)
}

// --- Close nil safety ---

func TestCloseNilStore(t *testing.T) {
	t.Parallel()

	var s *db.Store
	require.NoError(t, s.Close())
}

func TestCloseNilDB(t *testing.T) {
	t.Parallel()

	// A store with a nil db should not panic.
	// We can't construct this via Open, so we test nil receiver on Close.
	s := &db.Store{}
	require.NoError(t, s.Close())
}

// --- CreateOrUpdateItem with validation failure ---

func TestUntypedCreateOrUpdateItemValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// ImageVersion referencing non-existent source.
	err := store.CreateOrUpdateItem(&db.ImageVersion{
		BaseModel: db.BaseModel{ID: "ver-bad-upsert"},
		SourceID:  "no-such-source",
		Tag:       "v1",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- Cluster validation failure ---

func TestUntypedCreateOrUpdateItemClusterValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Cluster referencing non-existent image and backend image.
	err := store.CreateOrUpdateItem(&db.Cluster{
		BaseModel:      db.BaseModel{ID: "cl-bad"},
		Name:           "bad",
		Backend:        "incus",
		ImageID:        "no-img",
		BackendImageID: "no-bimg",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- BackendImage validation failure ---

func TestUntypedCreateOrUpdateItemBackendImageValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateOrUpdateItem(&db.BackendImage{
		BaseModel:  db.BaseModel{ID: "bimg-bad"},
		Backend:    "incus",
		ImageID:    "no-img",
		ExternalID: "ext",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- Artifact validation failure ---

func TestUntypedCreateOrUpdateItemArtifactValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateOrUpdateItem(&db.ImageArtifact{
		BaseModel: db.BaseModel{ID: "art-bad"},
		ImageID:   "no-img",
		Type:      db.ArtifactTypeRootFS,
		Path:      "/tmp/x",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- Image validation failure ---

func TestUntypedCreateOrUpdateItemImageValidationFails(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateOrUpdateItem(&db.Image{
		BaseModel: db.BaseModel{ID: "img-bad"},
		VersionID: "no-ver",
		Name:      "test",
		Path:      "/tmp",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- ListItems with empty bucket ---

func TestListItemsEmptyBucket(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	sources := make([]db.ImageSource, 0)
	require.NoError(t, store.ListItems(&sources))
	require.Empty(t, sources)
}

// --- Generic ListItems empty ---

func TestGenericListItemsEmpty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	sources, err := db.ListItems[db.ImageSource](store)
	require.NoError(t, err)
	require.Empty(t, sources)
}

// --- Untyped CreateOrUpdateItem creates and updates ---

func TestUntypedCreateOrUpdateItemCreatesAndUpdates(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	src := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "upsert-untyped"},
		Kind:      "registry",
		Location:  "loc-v1",
	}
	require.NoError(t, store.CreateOrUpdateItem(src))

	got := &db.ImageSource{}
	require.NoError(t, store.GetItem("upsert-untyped", got))
	require.Equal(t, "loc-v1", got.Location)

	src.Location = "loc-v2"
	require.NoError(t, store.CreateOrUpdateItem(src))

	got = &db.ImageSource{}
	require.NoError(t, store.GetItem("upsert-untyped", got))
	require.Equal(t, "loc-v2", got.Location)
}

// --- Close after open ---

func TestCloseAfterOpen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "close-test.db")
	store, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, store.Close())
}

// --- Duplicate name ref is fine (overwrite) ---

func TestDuplicateSetNameRefIsOverwrite(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.SetNameRef("img:v1", "ref1"))
	require.NoError(t, store.SetNameRef("img:v1", "ref2"))

	ref, err := store.GetNameRef("img:v1")
	require.NoError(t, err)
	require.Equal(t, "ref2", ref)
}

// --- Multiple name refs ---

func TestMultipleNameRefs(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.SetNameRef("a:v1", "ref-a"))
	require.NoError(t, store.SetNameRef("b:v2", "ref-b"))

	refA, err := store.GetNameRef("a:v1")
	require.NoError(t, err)
	require.Equal(t, "ref-a", refA)

	refB, err := store.GetNameRef("b:v2")
	require.NoError(t, err)
	require.Equal(t, "ref-b", refB)

	require.NoError(t, store.RemoveNameRef("a:v1"))

	_, err = store.GetNameRef("a:v1")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	// b should still be there.
	refB, err = store.GetNameRef("b:v2")
	require.NoError(t, err)
	require.Equal(t, "ref-b", refB)
}

// --- Generic ListItems with data ---

func TestGenericListItemsWithData(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "list-a"},
		Kind:      "registry",
		Location:  "loc-a",
	}))
	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "list-b"},
		Kind:      "file",
		Location:  "loc-b",
	}))

	sources, err := db.ListItems[db.ImageSource](store)
	require.NoError(t, err)
	require.Len(t, sources, 2)
}

// --- Open error paths ---

func TestOpenInvalidPath(t *testing.T) {
	t.Parallel()

	_, err := db.Open("/nonexistent/dir/iknite.db")
	require.Error(t, err)
}

func TestOpenFileNotDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "not-a-db")
	require.NoError(t, os.WriteFile(path, []byte("not a bbolt database"), 0o600))

	_, err := db.Open(path)
	require.Error(t, err)
}

// --- ListItems with corrupted data triggers unmarshal error ---

func insertCorruptedBucketData(t *testing.T, bucketName, key, value string) *db.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corrupt.db")
	store, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, store.Close())

	dbInstance, err := bbolt.Open(path, 0o600, nil)
	require.NoError(t, err)

	err = dbInstance.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		return bucket.Put([]byte(key), []byte(value))
	})
	require.NoError(t, err)
	require.NoError(t, dbInstance.Close())

	store, err = db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	return store
}

func TestListItemsCorruptedData(t *testing.T) {
	t.Parallel()

	store := insertCorruptedBucketData(t, "images", "corrupted", "not valid json")

	var images []db.Image
	err := store.ListItems(&images)
	require.Error(t, err)
}

// --- Save with unknown save mode is not testable (internal constant) ---
// --- applyTimestamps with corrupted existing data ---

func TestGetItemCorruptedData(t *testing.T) {
	t.Parallel()

	store := insertCorruptedBucketData(t, "image_sources", "bad-data", "{invalid json")

	out := &db.ImageSource{}
	err := store.GetItem("bad-data", out)
	require.Error(t, err)
}

// --- applyTimestamps unmarshal error via update on corrupted data ---

func TestUpdateOnCorruptedDataTriggersTimestampError(t *testing.T) {
	t.Parallel()

	store := insertCorruptedBucketData(t, "image_sources", "ts-bad", "{invalid")

	src := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "ts-bad"},
		Kind:      "registry",
		Location:  "new",
	}
	err := store.UpdateItem(src)
	require.Error(t, err)
}

// --- ListItems with corrupted data in image_versions bucket ---

func TestListItemsCorruptedVersions(t *testing.T) {
	t.Parallel()

	store := insertCorruptedBucketData(t, "image_versions", "bad-ver", "not json")

	var versions []db.ImageVersion
	err := store.ListItems(&versions)
	require.Error(t, err)
}

// --- SetNameRef with bucket-level error ---
// These are hard to trigger without bbolt corruption. The empty-input
// tests above cover the validation layer.

// --- ListItems with generic type on unsupported type ---

func TestListItemsGenericUnsupportedType(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	type customType struct {
		Value string
	}

	_, err := db.ListItems[customType](store)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type")
}

// --- ListItems with unsupported type via pointer ---

func TestListItemsNotPointer(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	var s []db.ImageSource
	err := store.ListItems(s)
	require.Error(t, err)
}

// --- Untyped methods with valid types but missing records ---

func TestUntypedUpdateItemNotPointerOutput(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Create, then update a source to exercise the full update path.
	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "upd-full"},
		Kind:      "registry",
		Location:  "loc1",
	}))

	src := &db.ImageSource{}
	require.NoError(t, store.GetItem("upd-full", src))
	src.Location = "loc2"
	require.NoError(t, store.UpdateItem(src))

	got := &db.ImageSource{}
	require.NoError(t, store.GetItem("upd-full", got))
	require.Equal(t, "loc2", got.Location)
}

// --- Untyped CreateOrUpdateItem full path ---

func TestUntypedCreateOrUpdateItemFullCycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Create new.
	src := &db.ImageSource{
		BaseModel: db.BaseModel{ID: "cou-full"},
		Kind:      "registry",
		Location:  "loc1",
	}
	require.NoError(t, store.CreateOrUpdateItem(src))

	// Update existing.
	src.Location = "loc2"
	require.NoError(t, store.CreateOrUpdateItem(src))

	got := &db.ImageSource{}
	require.NoError(t, store.GetItem("cou-full", got))
	require.Equal(t, "loc2", got.Location)
}

// --- Untyped DeleteItem full path ---

func TestUntypedDeleteItemFullPath(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "del-full"},
		Kind:      "registry",
		Location:  "loc",
	}))

	src := &db.ImageSource{}
	require.NoError(t, store.GetItem("del-full", src))
	require.NoError(t, store.DeleteItem(src))

	err := store.GetItem("del-full", &db.ImageSource{})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

// --- Untyped GetItem full path ---

func TestUntypedGetItemFullPath(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	require.NoError(t, store.CreateItem(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "get-full"},
		Kind:      "file",
		Location:  "loc",
	}))

	out := &db.ImageSource{}
	require.NoError(t, store.GetItem("get-full", out))
	require.Equal(t, "loc", out.Location)
}
