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
		store.CreateImageSource(&db.ImageSource{
			BaseModel: db.BaseModel{ID: graph.sourceID},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)
	require.NoError(
		t,
		store.CreateImageVersion(&db.ImageVersion{
			BaseModel: db.BaseModel{ID: graph.versionID},
			SourceID:  graph.sourceID,
			Tag:       "v0.7.1-devel-1.36.1",
		}),
	)
	require.NoError(
		t,
		store.CreateImage(&db.Image{
			BaseModel: db.BaseModel{ID: graph.imageID},
			VersionID: graph.versionID,
			Name:      "iknite-rootfs",
		}),
	)
	require.NoError(
		t,
		store.CreateImageArtifact(&db.ImageArtifact{
			BaseModel: db.BaseModel{ID: graph.artifactID},
			ImageID:   graph.imageID,
			Type:      db.ArtifactTypeRootFS,
			Path:      "/tmp/rootfs.tar.gz",
		}),
	)
	require.NoError(
		t,
		store.CreateBackendImage(&db.BackendImage{
			BaseModel:  db.BaseModel{ID: graph.backendImageID},
			Backend:    "incus",
			ImageID:    graph.imageID,
			ExternalID: "incus-image-01",
		}),
	)
	require.NoError(
		t,
		store.CreateCluster(&db.Cluster{
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
		store.CreateImageSource(&db.ImageSource{
			BaseModel: db.BaseModel{ID: "src"},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)

	sources, err := store.ListImageSources()
	require.NoError(t, err)
	require.Len(t, sources, 1)
}

func TestStoreCRUDLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	graph := seedDataGraph(t, store)

	source, err := store.GetImageSource(graph.sourceID)
	require.NoError(t, err)
	require.Equal(t, "registry", source.Kind)
	require.False(t, source.CreatedAt.IsZero())
	require.False(t, source.UpdatedAt.IsZero())

	version, err := store.GetImageVersion(graph.versionID)
	require.NoError(t, err)
	require.Equal(t, "v0.7.1-devel-1.36.1", version.Tag)
	require.False(t, version.CreatedAt.IsZero())
	require.False(t, version.UpdatedAt.IsZero())

	img, err := store.GetImage(graph.imageID)
	require.NoError(t, err)
	require.Equal(t, graph.versionID, img.VersionID)
	require.False(t, img.CreatedAt.IsZero())
	require.False(t, img.UpdatedAt.IsZero())
	previousCreatedAt := img.CreatedAt
	previousUpdatedAt := img.UpdatedAt
	time.Sleep(time.Millisecond)

	artifact, err := store.GetImageArtifact(graph.artifactID)
	require.NoError(t, err)
	require.Equal(t, db.ArtifactTypeRootFS, artifact.Type)

	backendImage, err := store.GetBackendImage(graph.backendImageID)
	require.NoError(t, err)
	require.Equal(t, "incus", backendImage.Backend)

	cluster, err := store.GetCluster(graph.clusterID)
	require.NoError(t, err)
	require.Equal(t, "dev", cluster.Name)

	img.Name = "iknite-rootfs-updated"
	require.NoError(t, store.UpdateImage(img))
	imgAfterUpdate, err := store.GetImage(graph.imageID)
	require.NoError(t, err)
	require.Equal(t, previousCreatedAt, imgAfterUpdate.CreatedAt)
	require.True(t, imgAfterUpdate.UpdatedAt.After(previousUpdatedAt))
	require.Equal(t, "iknite-rootfs-updated", imgAfterUpdate.Name)

	cluster.Workspace = "/workspace/my-repo"
	require.NoError(t, store.UpdateCluster(cluster))
	clusterAfterUpdate, err := store.GetCluster(graph.clusterID)
	require.NoError(t, err)
	require.False(t, clusterAfterUpdate.CreatedAt.IsZero())
	require.False(t, clusterAfterUpdate.UpdatedAt.IsZero())

	images, err := store.ListImages()
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, "iknite-rootfs-updated", images[0].Name)

	clusters, err := store.ListClusters()
	require.NoError(t, err)
	require.Len(t, clusters, 1)
	require.Equal(t, "/workspace/my-repo", clusters[0].Workspace)

	require.NoError(t, store.DeleteCluster(graph.clusterID))
	require.NoError(t, store.DeleteBackendImage(graph.backendImageID))
	require.NoError(t, store.DeleteImageArtifact(graph.artifactID))
	require.NoError(t, store.DeleteImage(graph.imageID))
	require.NoError(t, store.DeleteImageVersion(graph.versionID))
	require.NoError(t, store.DeleteImageSource(graph.sourceID))

	_, err = store.GetCluster(graph.clusterID)
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestStoreValidatesReferences(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CreateImageVersion(&db.ImageVersion{
		BaseModel: db.BaseModel{ID: "ver-missing"},
		SourceID:  "missing-source",
		Tag:       "v1",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = store.CreateImage(&db.Image{BaseModel: db.BaseModel{ID: "img-missing"}, VersionID: "missing-version"})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = store.CreateBackendImage(&db.BackendImage{
		BaseModel: db.BaseModel{ID: "bimg-missing"},
		Backend:   "incus",
		ImageID:   "missing-image",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = store.CreateCluster(&db.Cluster{
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
		store.CreateImageSource(&db.ImageSource{
			BaseModel: db.BaseModel{ID: "src-1"},
			Kind:      "registry",
			Location:  "ghcr.io/kaweezle/iknite",
		}),
	)
	err := store.CreateImageSource(&db.ImageSource{
		BaseModel: db.BaseModel{ID: "src-1"},
		Kind:      "registry",
		Location:  "ghcr.io/kaweezle/iknite",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrAlreadyExists)

	generated := &db.ImageSource{BaseModel: db.BaseModel{}, Kind: "registry"}
	err = store.CreateImageSource(generated)
	require.NoError(t, err)
	require.NotEmpty(t, generated.ID)
	parsed, err := uuid.Parse(generated.ID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())

	err = store.DeleteImageSource("missing")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrNotFound)

	err = store.DeleteImageSource("")
	require.Error(t, err)
	require.ErrorIs(t, err, db.ErrInvalidID)
}
