// cSpell: words imagesvc
package image

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	imageMock "github.com/kaweezle/iknite/mocks/pkg/iknitectl/image"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/testutil"
)

// mockMetadataStore implements MetadataStore for testing.
type mockMetadataStore struct {
	onGetNameRef         func(string) (string, error)
	onGetItem            func(string, any) error
	onListItems          func(any) error
	onDeleteItem         func(db.IDAccessor) error
	onRemoveNameRef      func(string) error
	onSetNameRef         func(string, string) error
	onCreateItem         func(db.IDAccessor) error
	onUpdateItem         func(db.IDAccessor) error
	onCreateOrUpdateItem func(db.IDAccessor) error
}

func (m *mockMetadataStore) GetNameRef(name string) (string, error) {
	if m.onGetNameRef != nil {
		return m.onGetNameRef(name)
	}
	return "", fmt.Errorf("GetNameRef not implemented")
}

func (m *mockMetadataStore) GetItem(id string, out any) error {
	if m.onGetItem != nil {
		return m.onGetItem(id, out)
	}
	return fmt.Errorf("GetItem not implemented")
}

func (m *mockMetadataStore) ListItems(out any) error {
	if m.onListItems != nil {
		return m.onListItems(out)
	}
	return fmt.Errorf("ListItems not implemented")
}

func (m *mockMetadataStore) DeleteItem(item db.IDAccessor) error {
	if m.onDeleteItem != nil {
		return m.onDeleteItem(item)
	}
	return fmt.Errorf("DeleteItem not implemented")
}

func (m *mockMetadataStore) RemoveNameRef(name string) error {
	if m.onRemoveNameRef != nil {
		return m.onRemoveNameRef(name)
	}
	return fmt.Errorf("RemoveNameRef not implemented")
}

func (m *mockMetadataStore) SetNameRef(name, ref string) error {
	if m.onSetNameRef != nil {
		return m.onSetNameRef(name, ref)
	}
	return fmt.Errorf("SetNameRef not implemented")
}

func (m *mockMetadataStore) CreateItem(item db.IDAccessor) error {
	if m.onCreateItem != nil {
		return m.onCreateItem(item)
	}
	return fmt.Errorf("CreateItem not implemented")
}

func (m *mockMetadataStore) UpdateItem(item db.IDAccessor) error {
	if m.onUpdateItem != nil {
		return m.onUpdateItem(item)
	}
	return fmt.Errorf("UpdateItem not implemented")
}

func (m *mockMetadataStore) CreateOrUpdateItem(item db.IDAccessor) error {
	if m.onCreateOrUpdateItem != nil {
		return m.onCreateOrUpdateItem(item)
	}
	return fmt.Errorf("CreateOrUpdateItem not implemented")
}

// trackingFS wraps a FileEnvironment and records RemoveAll calls.
type trackingFS struct {
	host.FileEnvironment
	removeAllCalls []string
}

func (f *trackingFS) RemoveAll(path string) error {
	f.removeAllCalls = append(f.removeAllCalls, path)
	err := f.FileEnvironment.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("trackingFS.RemoveAll: %w", err)
	}
	return nil
}

// failingFS wraps a FileEnvironment and returns an error for RemoveAll.
type failingFS struct {
	host.FileEnvironment
	removeAllErr error
}

func (f *failingFS) RemoveAll(_ string) error {
	return f.removeAllErr
}

// newTestService creates a Service with mocks for testing.
func newTestService(t *testing.T, store MetadataStore) *Service {
	t.Helper()
	h := testutil.NewDummyUserHost()
	c := &config.Config{}
	require.NoError(t, config.NewConfigOptions(h).Resolve(h, c))
	return &Service{
		FS:     h,
		Logger: testutil.TestLogger(t),
		Config: c,
		Store:  store,
	}
}

// newTestServiceWithFS creates a Service with a custom FS for testing.
func newTestServiceWithFS(
	t *testing.T,
	store MetadataStore,
	fs host.FileEnvironment,
) *Service {
	t.Helper()
	h := testutil.NewDummyUserHost()
	c := &config.Config{}
	require.NoError(t, config.NewConfigOptions(h).Resolve(h, c))
	return &Service{
		FS:     fs,
		Logger: testutil.TestLogger(t),
		Config: c,
		Store:  store,
	}
}

// --- Info tests ---

func TestInfo_Success(t *testing.T) {
	t.Parallel()

	s := imageMock.NewMockMetadataStore(t)
	s.EXPECT().GetNameRef("iknite:latest").Return("ghcr.io/kaweezle/iknite@latest", nil)
	s.EXPECT().GetItem(
		"ghcr.io/kaweezle/iknite@latest",
		mock.AnythingOfType(reflect.TypeFor[*db.Image]().String()),
	).RunAndReturn(func(id string, out any) error {
		if o, ok := out.(*db.Image); ok {
			*o = db.Image{
				BaseModel: db.BaseModel{ID: id},
				VersionID: "ghcr.io/kaweezle/iknite@latest",
				Name:      "iknite:latest",
				Path:      "/tmp/images/test",
			}
			return nil
		}
		return fmt.Errorf("unexpected type for GetItem: %T", out)
	})
	s.EXPECT().GetItem(
		"ghcr.io/kaweezle/iknite@latest",
		mock.AnythingOfType(reflect.TypeFor[*db.ImageVersion]().String()),
	).RunAndReturn(func(id string, out any) error {
		if o, ok := out.(*db.ImageVersion); ok {
			*o = db.ImageVersion{
				BaseModel:         db.BaseModel{ID: id},
				SourceID:          "ghcr.io/kaweezle/iknite",
				Tag:               "latest",
				ManifestDigest:    "sha256:abc123",
				ManifestMediaType: "application/vnd.oci.image.manifest.v1+json",
			}
			return nil
		}
		return fmt.Errorf("unexpected type for GetItem: %T", out)
	})
	s.EXPECT().GetItem(
		"ghcr.io/kaweezle/iknite",
		mock.AnythingOfType(reflect.TypeFor[*db.ImageSource]().String()),
	).RunAndReturn(func(id string, out any) error {
		if o, ok := out.(*db.ImageSource); ok {
			*o = db.ImageSource{
				BaseModel: db.BaseModel{ID: id},
				Kind:      "registry",
				Location:  "ghcr.io/kaweezle/iknite",
			}
			return nil
		}
		return fmt.Errorf("unexpected type for GetItem: %T", out)
	})
	s.EXPECT().ListItems(
		mock.AnythingOfType(reflect.TypeFor[*[]db.ImageArtifact]().String()),
	).RunAndReturn(func(out any) error {
		if artifacts, ok := out.(*[]db.ImageArtifact); ok {
			*artifacts = []db.ImageArtifact{
				{
					BaseModel: db.BaseModel{ID: "art-1"},
					ImageID:   "ghcr.io/kaweezle/iknite@latest",
					Path:      "/tmp/images/test/rootfs.tar.gz",
					Digest:    "sha256:def456",
					Type:      db.ArtifactTypeRootFS,
					Size:      1024,
				},
			}
			return nil
		}
		return fmt.Errorf("unexpected type for ListItems: %T", out)
	})

	svc := newTestService(t, s)
	info, err := svc.Info("iknite:latest")
	require.NoError(t, err)
	require.Equal(t, "iknite:latest", info.Name)
	require.Equal(t, "/tmp/images/test", info.Path)
	require.Equal(t, "latest", info.Reference)
	require.Equal(t, "ghcr.io/kaweezle/iknite", info.Source.ID)
	require.Equal(t, "registry", info.Source.Kind)
	require.Equal(t, "ghcr.io/kaweezle/iknite", info.Source.Location)
	require.Equal(t, "sha256:abc123", info.Manifest.Digest)
	require.Equal(t, "application/vnd.oci.image.manifest.v1+json", info.Manifest.MediaType)
	require.Len(t, info.Artifacts, 1)
	require.Equal(t, "sha256:def456", info.Artifacts[0].Digest)
	require.Equal(t, db.ArtifactTypeRootFS, info.Artifacts[0].Type)
	require.EqualValues(t, 1024, info.TotalSize)
}

func TestInfo_NilFS(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	_, err := svc.Info("test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "filesystem dependency is required")
}

func TestInfo_NilStore(t *testing.T) {
	t.Parallel()

	h := testutil.NewDummyUserHost()
	svc := &Service{FS: h}
	_, err := svc.Info("test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "store dependency is required")
}

func TestInfo_NameNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "", db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	_, err := svc.Info("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to resolve image name")
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestInfo_ImageNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(_ string, _ any) error {
			return db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	_, err := svc.Info("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get image record")
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestInfo_VersionNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
				}
				return nil
			case *db.ImageVersion:
				return db.ErrNotFound
			}
			return db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	_, err := svc.Info("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get image version")
}

func TestInfo_SourceNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			case *db.ImageSource:
				return db.ErrNotFound
			}
			return db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	_, err := svc.Info("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get image source")
}

func TestInfo_ArtifactsListError(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			case *db.ImageSource:
				*o = db.ImageSource{
					BaseModel: db.BaseModel{ID: id},
					Kind:      "registry",
					Location:  "ghcr.io/kaweezle/iknite",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(_ any) error {
			return fmt.Errorf("database read error")
		},
	}

	svc := newTestService(t, store)
	_, err := svc.Info("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to list artifacts")
}

// --- Remove tests ---

func TestRemove_Success(t *testing.T) {
	t.Parallel()

	var deletedItems []string
	var removedNameRefs []string

	h := testutil.NewDummyUserHost()
	fs := &trackingFS{FileEnvironment: h}

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "/tmp/images/test",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(out any) error {
			if artifacts, ok := out.(*[]db.ImageArtifact); ok {
				*artifacts = []db.ImageArtifact{
					{
						BaseModel: db.BaseModel{ID: "art-1"},
						ImageID:   "ghcr.io/kaweezle/iknite@latest",
						Path:      "/tmp/images/test/rootfs.tar.gz",
						Type:      db.ArtifactTypeRootFS,
					},
				}
			}
			return nil
		},
		onDeleteItem: func(item db.IDAccessor) error {
			deletedItems = append(deletedItems, item.GetID())
			return nil
		},
		onRemoveNameRef: func(name string) error {
			removedNameRefs = append(removedNameRefs, name)
			return nil
		},
	}

	svc := newTestServiceWithFS(t, store, fs)
	err := svc.Remove("iknite:latest")
	require.NoError(t, err)

	require.Len(t, deletedItems, 3) // artifact, image, version
	require.Contains(t, deletedItems, "art-1")
	require.Len(t, removedNameRefs, 1)
	require.Equal(t, "iknite:latest", removedNameRefs[0])
	require.Len(t, fs.removeAllCalls, 1)
	require.Equal(t, "/tmp/images/test", fs.removeAllCalls[0])
}

func TestRemove_NilFS(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	err := svc.Remove("test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "filesystem dependency is required")
}

func TestRemove_NilStore(t *testing.T) {
	t.Parallel()

	h := testutil.NewDummyUserHost()
	svc := &Service{FS: h}
	err := svc.Remove("test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "store dependency is required")
}

func TestRemove_NameNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "", db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	err := svc.Remove("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to resolve image name")
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestRemove_ImageNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(_ string, _ any) error {
			return db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get image record")
}

func TestRemove_VersionNotFound(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
				}
				return nil
			case *db.ImageVersion:
				return db.ErrNotFound
			}
			return db.ErrNotFound
		},
	}

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get image version")
}

func TestRemove_ArtifactsListError(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(_ any) error {
			return fmt.Errorf("database read error")
		},
	}

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to list artifacts")
}

func TestRemove_DeleteArtifactError(t *testing.T) {
	t.Parallel()

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "/tmp/images/test",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(out any) error {
			if artifacts, ok := out.(*[]db.ImageArtifact); ok {
				*artifacts = []db.ImageArtifact{
					{
						BaseModel: db.BaseModel{ID: "art-1"},
						ImageID:   "ghcr.io/kaweezle/iknite@latest",
						Type:      db.ArtifactTypeRootFS,
					},
				}
			}
			return nil
		},
		onDeleteItem: func(_ db.IDAccessor) error {
			return fmt.Errorf("database delete error")
		},
	}

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete artifact")
}

func deleteMockMetadataStore(t *testing.T, deletedItems *[]string, okDeletedCount int) *mockMetadataStore {
	t.Helper()
	return &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "/tmp/images/test",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(out any) error {
			if artifacts, ok := out.(*[]db.ImageArtifact); ok {
				*artifacts = []db.ImageArtifact{
					{
						BaseModel: db.BaseModel{ID: "art-1"},
						ImageID:   "ghcr.io/kaweezle/iknite@latest",
						Type:      db.ArtifactTypeRootFS,
					},
				}
			}
			return nil
		},
		onDeleteItem: func(item db.IDAccessor) error {
			*deletedItems = append(*deletedItems, item.GetID())
			if len(*deletedItems) <= okDeletedCount {
				return nil // artifact succeeded
			}
			return fmt.Errorf("database delete error")
		},
	}
}

func TestRemove_DeleteImageError(t *testing.T) {
	t.Parallel()

	var deletedItems []string

	store := deleteMockMetadataStore(t, &deletedItems, 1)

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete image record")
	require.Len(t, deletedItems, 2) // artifact + image (failed)
}

func TestRemove_DeleteVersionError(t *testing.T) {
	t.Parallel()

	var deletedItems []string

	store := deleteMockMetadataStore(t, &deletedItems, 2)

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete version record")
	require.Len(t, deletedItems, 3) // artifact + image + version (failed)
}

func TestRemove_RemoveNameRefError(t *testing.T) {
	t.Parallel()

	var deletedItems []string

	store := deleteMockMetadataStore(t, &deletedItems, 15) // all deletes succeed
	store.onRemoveNameRef = func(_ string) error {
		return fmt.Errorf("database delete error")
	}

	svc := newTestService(t, store)
	err := svc.Remove("iknite:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete name reference")
	require.Len(t, deletedItems, 3) // all deletes succeeded
}

func TestRemoveRemoveAllErrorIsNonFatal(t *testing.T) {
	t.Parallel()

	var deletedItems []string
	var removedNameRefs []string

	h := testutil.NewDummyUserHost()
	fs := &failingFS{
		FileEnvironment: h,
		removeAllErr:    fmt.Errorf("permission denied"),
	}

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "/tmp/images/test",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(_ any) error {
			return nil
		},
		onDeleteItem: func(item db.IDAccessor) error {
			deletedItems = append(deletedItems, item.GetID())
			return nil
		},
		onRemoveNameRef: func(name string) error {
			removedNameRefs = append(removedNameRefs, name)
			return nil
		},
	}

	svc := newTestServiceWithFS(t, store, fs)
	err := svc.Remove("iknite:latest")
	require.NoError(t, err)

	require.Len(t, deletedItems, 2) // image + version (no artifacts)
	require.Len(t, removedNameRefs, 1)
}

func TestRemoveNoArtifacts(t *testing.T) {
	t.Parallel()

	var deletedItems []string

	h := testutil.NewDummyUserHost()
	fs := &trackingFS{FileEnvironment: h}

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "/tmp/images/test",
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(out any) error {
			if artifacts, ok := out.(*[]db.ImageArtifact); ok {
				*artifacts = []db.ImageArtifact{}
			}
			return nil
		},
		onDeleteItem: func(item db.IDAccessor) error {
			deletedItems = append(deletedItems, item.GetID())
			return nil
		},
		onRemoveNameRef: func(_ string) error {
			return nil
		},
	}

	svc := newTestServiceWithFS(t, store, fs)
	err := svc.Remove("iknite:latest")
	require.NoError(t, err)

	require.Len(t, deletedItems, 2) // image + version only
	require.Len(t, fs.removeAllCalls, 1)
}

func TestRemoveEmptyPath(t *testing.T) {
	t.Parallel()

	h := testutil.NewDummyUserHost()
	fs := &trackingFS{FileEnvironment: h}

	store := &mockMetadataStore{
		onGetNameRef: func(_ string) (string, error) {
			return "ghcr.io/kaweezle/iknite@latest", nil
		},
		onGetItem: func(id string, out any) error {
			switch o := out.(type) {
			case *db.Image:
				*o = db.Image{
					BaseModel: db.BaseModel{ID: id},
					VersionID: "ghcr.io/kaweezle/iknite@latest",
					Name:      "iknite:latest",
					Path:      "", // empty path
				}
				return nil
			case *db.ImageVersion:
				*o = db.ImageVersion{
					BaseModel: db.BaseModel{ID: id},
					SourceID:  "ghcr.io/kaweezle/iknite",
					Tag:       "latest",
				}
				return nil
			}
			return db.ErrNotFound
		},
		onListItems: func(_ any) error {
			return nil
		},
		onDeleteItem: func(_ db.IDAccessor) error {
			return nil
		},
		onRemoveNameRef: func(_ string) error {
			return nil
		},
	}

	svc := newTestServiceWithFS(t, store, fs)
	err := svc.Remove("iknite:latest")
	require.NoError(t, err)

	require.Empty(t, fs.removeAllCalls) // RemoveAll should not be called
}
