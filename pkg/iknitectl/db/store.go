// cSpell: words bbolt wrapcheck
package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	bbolt "go.etcd.io/bbolt"
)

var (
	// ErrNotFound is returned when a record cannot be found.
	ErrNotFound = errors.New("record not found")
	// ErrAlreadyExists is returned when creating a record with an existing ID.
	ErrAlreadyExists = errors.New("record already exists")
	// ErrInvalidID is returned when a record ID is empty.
	ErrInvalidID = errors.New("invalid id")
)

var (
	bucketImageSources   = []byte("image_sources")
	bucketImageVersions  = []byte("image_versions")
	bucketImages         = []byte("images")
	bucketImageArtifacts = []byte("image_artifacts")
	bucketBackendImages  = []byte("backend_images")
	bucketClusters       = []byte("clusters")
)

// Store provides bbolt-backed persistence for iknitectl client objects.
type Store struct {
	db *bbolt.DB
}

// Open opens the database and initializes all required buckets.
func Open(path string) (*Store, error) {
	database, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: database}
	if err = store.ensureBuckets(); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize buckets: %w", errors.Join(err, closeErr))
		}
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return store, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close() //nolint:wrapcheck // Preserve bbolt close error.
}

func (s *Store) ensureBuckets() error {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{
			bucketImageSources,
			bucketImageVersions,
			bucketImages,
			bucketImageArtifacts,
			bucketBackendImages,
			bucketClusters,
		} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %q: %w", string(bucket), err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to ensure buckets: %w", err)
	}

	return nil
}

func requireID(id string) error {
	if id == "" {
		return ErrInvalidID
	}
	return nil
}

func getBucket(tx *bbolt.Tx, name []byte) (*bbolt.Bucket, error) {
	bucket := tx.Bucket(name)
	if bucket == nil {
		return nil, fmt.Errorf("missing bucket %q", string(name))
	}
	return bucket, nil
}

func requireReference(tx *bbolt.Tx, bucketName []byte, id, recordKind string) error {
	if err := requireID(id); err != nil {
		return fmt.Errorf("invalid %s id: %w", recordKind, err)
	}
	bucket, err := getBucket(tx, bucketName)
	if err != nil {
		return err
	}
	if bucket.Get([]byte(id)) == nil {
		return fmt.Errorf("%w: %s %q", ErrNotFound, recordKind, id)
	}
	return nil
}

func applyTimestamps(value any, mustExist bool, existing []byte, now time.Time) error {
	accessor, ok := value.(TimestampAccessor)
	if !ok {
		return nil
	}

	if !mustExist {
		accessor.SetCreatedAt(now)
		accessor.SetUpdatedAt(now)
		return nil
	}

	var existingTimestamp struct {
		CreatedAt time.Time `json:"createdAt"`
	}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &existingTimestamp); err != nil {
			return fmt.Errorf("failed to decode existing timestamp: %w", err)
		}
	}

	if existingTimestamp.CreatedAt.IsZero() {
		existingTimestamp.CreatedAt = now
	}
	accessor.SetCreatedAt(existingTimestamp.CreatedAt)
	accessor.SetUpdatedAt(now)

	return nil
}

func ensureRecordID(value IDAccessor) (string, error) {
	id := value.GetID()
	if id != "" {
		return id, nil
	}

	generated, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate id: %w", err)
	}
	id = generated.String()
	value.SetID(id)

	return id, nil
}

func (s *Store) save(
	bucketName []byte,
	value IDAccessor,
	validate func(tx *bbolt.Tx) error,
	mustExist bool,
) error {
	id, resolveErr := ensureRecordID(value)
	if resolveErr != nil {
		return resolveErr
	}

	idErr := requireID(id)
	if idErr != nil {
		return idErr
	}

	persistErr := s.db.Update(func(tx *bbolt.Tx) error {
		if validate != nil {
			validateErr := validate(tx)
			if validateErr != nil {
				return validateErr
			}
		}

		bucket, bucketErr := getBucket(tx, bucketName)
		if bucketErr != nil {
			return bucketErr
		}

		existing := bucket.Get([]byte(id))
		if mustExist && existing == nil {
			return fmt.Errorf("%w: %s %q", ErrNotFound, string(bucketName), id)
		}
		if !mustExist && existing != nil {
			return fmt.Errorf("%w: %s %q", ErrAlreadyExists, string(bucketName), id)
		}

		timestampErr := applyTimestamps(value, mustExist, existing, time.Now().UTC())
		if timestampErr != nil {
			return fmt.Errorf("failed to set timestamps: %w", timestampErr)
		}

		payload, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal %s %q: %w", string(bucketName), id, err)
		}

		action := "save"
		if mustExist {
			action = "update"
		}
		if err = bucket.Put([]byte(id), payload); err != nil {
			return fmt.Errorf("failed to %s %s %q: %w", action, string(bucketName), id, err)
		}
		return nil
	})
	if persistErr != nil {
		return fmt.Errorf("failed to persist %s %q: %w", string(bucketName), id, persistErr)
	}

	return nil
}

func (s *Store) create(bucketName []byte, value IDAccessor, validate func(tx *bbolt.Tx) error) error {
	return s.save(bucketName, value, validate, false)
}

func (s *Store) get(bucketName []byte, id string, out any) error {
	if err := requireID(id); err != nil {
		return err
	}

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketName)
		if err != nil {
			return err
		}
		payload := bucket.Get([]byte(id))
		if payload == nil {
			return fmt.Errorf("%w: %s %q", ErrNotFound, string(bucketName), id)
		}
		if err = json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("failed to decode %s %q: %w", string(bucketName), id, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to read %s %q: %w", string(bucketName), id, err)
	}

	return nil
}

func (s *Store) update(bucketName []byte, value IDAccessor, validate func(tx *bbolt.Tx) error) error {
	return s.save(bucketName, value, validate, true)
}

func (s *Store) delete(bucketName []byte, id string) error {
	if err := requireID(id); err != nil {
		return err
	}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketName)
		if err != nil {
			return err
		}
		if bucket.Get([]byte(id)) == nil {
			return fmt.Errorf("%w: %s %q", ErrNotFound, string(bucketName), id)
		}
		if err = bucket.Delete([]byte(id)); err != nil {
			return fmt.Errorf("failed to delete %s %q: %w", string(bucketName), id, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to remove %s %q: %w", string(bucketName), id, err)
	}

	return nil
}

func list[T any](s *Store, bucketName []byte) ([]T, error) {
	result := make([]T, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketName)
		if err != nil {
			return err
		}
		return bucket.ForEach(func(_, value []byte) error {
			var item T
			if err = json.Unmarshal(value, &item); err != nil {
				return fmt.Errorf("failed to decode %s record: %w", string(bucketName), err)
			}
			result = append(result, item)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", string(bucketName), err)
	}
	return result, nil
}

// CreateImageSource stores a new image source.
func (s *Store) CreateImageSource(item *ImageSource) error {
	if item == nil {
		return fmt.Errorf("image source is required")
	}
	return s.create(bucketImageSources, item, nil)
}

// GetImageSource fetches an image source by ID.
func (s *Store) GetImageSource(id string) (*ImageSource, error) {
	item := &ImageSource{}
	if err := s.get(bucketImageSources, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateImageSource updates an existing image source.
func (s *Store) UpdateImageSource(item *ImageSource) error {
	if item == nil {
		return fmt.Errorf("image source is required")
	}
	return s.update(bucketImageSources, item, nil)
}

// DeleteImageSource deletes an image source by ID.
func (s *Store) DeleteImageSource(id string) error {
	return s.delete(bucketImageSources, id)
}

// ListImageSources lists all image sources.
func (s *Store) ListImageSources() ([]ImageSource, error) {
	return list[ImageSource](s, bucketImageSources)
}

// CreateImageVersion stores a new image version.
func (s *Store) CreateImageVersion(item *ImageVersion) error {
	if item == nil {
		return fmt.Errorf("image version is required")
	}
	return s.create(bucketImageVersions, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImageSources, item.SourceID, "image source")
	})
}

// GetImageVersion fetches an image version by ID.
func (s *Store) GetImageVersion(id string) (*ImageVersion, error) {
	item := &ImageVersion{}
	if err := s.get(bucketImageVersions, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateImageVersion updates an existing image version.
func (s *Store) UpdateImageVersion(item *ImageVersion) error {
	if item == nil {
		return fmt.Errorf("image version is required")
	}
	return s.update(bucketImageVersions, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImageSources, item.SourceID, "image source")
	})
}

// DeleteImageVersion deletes an image version by ID.
func (s *Store) DeleteImageVersion(id string) error {
	return s.delete(bucketImageVersions, id)
}

// ListImageVersions lists all image versions.
func (s *Store) ListImageVersions() ([]ImageVersion, error) {
	return list[ImageVersion](s, bucketImageVersions)
}

// CreateImage stores a new image.
func (s *Store) CreateImage(item *Image) error {
	if item == nil {
		return fmt.Errorf("image is required")
	}
	return s.create(bucketImages, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImageVersions, item.VersionID, "image version")
	})
}

// GetImage fetches an image by ID.
func (s *Store) GetImage(id string) (*Image, error) {
	item := &Image{}
	if err := s.get(bucketImages, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateImage updates an existing image.
func (s *Store) UpdateImage(item *Image) error {
	if item == nil {
		return fmt.Errorf("image is required")
	}
	return s.update(bucketImages, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImageVersions, item.VersionID, "image version")
	})
}

// DeleteImage deletes an image by ID.
func (s *Store) DeleteImage(id string) error {
	return s.delete(bucketImages, id)
}

// ListImages lists all images.
func (s *Store) ListImages() ([]Image, error) {
	return list[Image](s, bucketImages)
}

// CreateImageArtifact stores a new image artifact.
func (s *Store) CreateImageArtifact(item *ImageArtifact) error {
	if item == nil {
		return fmt.Errorf("image artifact is required")
	}
	return s.create(bucketImageArtifacts, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImages, item.ImageID, "image")
	})
}

// GetImageArtifact fetches an image artifact by ID.
func (s *Store) GetImageArtifact(id string) (*ImageArtifact, error) {
	item := &ImageArtifact{}
	if err := s.get(bucketImageArtifacts, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateImageArtifact updates an existing image artifact.
func (s *Store) UpdateImageArtifact(item *ImageArtifact) error {
	if item == nil {
		return fmt.Errorf("image artifact is required")
	}
	return s.update(bucketImageArtifacts, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImages, item.ImageID, "image")
	})
}

// DeleteImageArtifact deletes an image artifact by ID.
func (s *Store) DeleteImageArtifact(id string) error {
	return s.delete(bucketImageArtifacts, id)
}

// ListImageArtifacts lists all image artifacts.
func (s *Store) ListImageArtifacts() ([]ImageArtifact, error) {
	return list[ImageArtifact](s, bucketImageArtifacts)
}

// CreateBackendImage stores a new backend image.
func (s *Store) CreateBackendImage(item *BackendImage) error {
	if item == nil {
		return fmt.Errorf("backend image is required")
	}
	return s.create(bucketBackendImages, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImages, item.ImageID, "image")
	})
}

// GetBackendImage fetches a backend image by ID.
func (s *Store) GetBackendImage(id string) (*BackendImage, error) {
	item := &BackendImage{}
	if err := s.get(bucketBackendImages, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateBackendImage updates an existing backend image.
func (s *Store) UpdateBackendImage(item *BackendImage) error {
	if item == nil {
		return fmt.Errorf("backend image is required")
	}
	return s.update(bucketBackendImages, item, func(tx *bbolt.Tx) error {
		return requireReference(tx, bucketImages, item.ImageID, "image")
	})
}

// DeleteBackendImage deletes a backend image by ID.
func (s *Store) DeleteBackendImage(id string) error {
	return s.delete(bucketBackendImages, id)
}

// ListBackendImages lists all backend images.
func (s *Store) ListBackendImages() ([]BackendImage, error) {
	return list[BackendImage](s, bucketBackendImages)
}

// CreateCluster stores a new cluster.
func (s *Store) CreateCluster(item *Cluster) error {
	if item == nil {
		return fmt.Errorf("cluster is required")
	}
	return s.create(bucketClusters, item, func(tx *bbolt.Tx) error {
		if err := requireReference(tx, bucketImages, item.ImageID, "image"); err != nil {
			return err
		}
		return requireReference(tx, bucketBackendImages, item.BackendImageID, "backend image")
	})
}

// GetCluster fetches a cluster by ID.
func (s *Store) GetCluster(id string) (*Cluster, error) {
	item := &Cluster{}
	if err := s.get(bucketClusters, id, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateCluster updates an existing cluster.
func (s *Store) UpdateCluster(item *Cluster) error {
	if item == nil {
		return fmt.Errorf("cluster is required")
	}
	return s.update(bucketClusters, item, func(tx *bbolt.Tx) error {
		if err := requireReference(tx, bucketImages, item.ImageID, "image"); err != nil {
			return err
		}
		return requireReference(tx, bucketBackendImages, item.BackendImageID, "backend image")
	})
}

// DeleteCluster deletes a cluster by ID.
func (s *Store) DeleteCluster(id string) error {
	return s.delete(bucketClusters, id)
}

// ListClusters lists all clusters.
func (s *Store) ListClusters() ([]Cluster, error) {
	return list[Cluster](s, bucketClusters)
}
