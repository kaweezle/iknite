// cSpell: words bbolt wrapcheck
package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	bucketImageNameRefs  = []byte("image_name_refs")
	bucketBackendImages  = []byte("backend_images")
	bucketClusters       = []byte("clusters")
)

type validationFunc func(tx *bbolt.Tx, item any) error

type saveMode int

const (
	saveModeCreate saveMode = iota
	saveModeUpdate
	saveModeCreateOrUpdate
)

type IDAccessorPointer[M any] interface {
	*M
	IDAccessor
}

type typeStoreParameters struct {
	validate   validationFunc
	bucketName []byte
}

var typeParameters = map[reflect.Type]typeStoreParameters{
	reflect.TypeFor[*ImageSource](): {
		bucketName: bucketImageSources,
		validate:   nil,
	},
	reflect.TypeFor[*ImageVersion](): {
		bucketName: bucketImageVersions,
		validate: func(tx *bbolt.Tx, item any) error {
			version, ok := item.(*ImageVersion)
			if !ok {
				return fmt.Errorf("invalid type for validation: %T", item)
			}
			return requireReference(tx, bucketImageSources, version.SourceID, "image source")
		},
	},
	reflect.TypeFor[*Image](): {
		bucketName: bucketImages,
		validate: func(tx *bbolt.Tx, item any) error {
			image, ok := item.(*Image)
			if !ok {
				return fmt.Errorf("invalid type for validation: %T", item)
			}
			if err := requireReference(tx, bucketImageVersions, image.VersionID, "image version"); err != nil {
				return err
			}
			return nil
		},
	},
	reflect.TypeFor[*ImageArtifact](): {
		bucketName: bucketImageArtifacts,
		validate: func(tx *bbolt.Tx, item any) error {
			artifact, ok := item.(*ImageArtifact)
			if !ok {
				return fmt.Errorf("invalid type for validation: %T", item)
			}
			if err := requireReference(tx, bucketImages, artifact.ImageID, "image"); err != nil {
				return err
			}
			return nil
		},
	},
	reflect.TypeFor[*BackendImage](): {
		bucketName: bucketBackendImages,
		validate: func(tx *bbolt.Tx, item any) error {
			backendImage, ok := item.(*BackendImage)
			if !ok {
				return fmt.Errorf("invalid type for validation: %T", item)
			}
			if err := requireReference(tx, bucketImages, backendImage.ImageID, "image"); err != nil {
				return err
			}
			return nil
		},
	},
	reflect.TypeFor[*Cluster](): {
		bucketName: bucketClusters,
		validate: func(tx *bbolt.Tx, item any) error {
			cluster, ok := item.(*Cluster)
			if !ok {
				return fmt.Errorf("invalid type for validation: %T", item)
			}
			if err := requireReference(tx, bucketImages, cluster.ImageID, "image"); err != nil {
				return err
			}
			if err := requireReference(tx, bucketBackendImages, cluster.BackendImageID, "backend image"); err != nil {
				return err
			}
			return nil
		},
	},
}

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
		for _, params := range typeParameters {
			if _, err := tx.CreateBucketIfNotExists(params.bucketName); err != nil {
				return fmt.Errorf("failed to create bucket %q: %w", string(params.bucketName), err)
			}
		}
		if _, err := tx.CreateBucketIfNotExists(bucketImageNameRefs); err != nil {
			return fmt.Errorf("failed to create bucket %q: %w", string(bucketImageNameRefs), err)
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

//nolint:gocyclo // save centralizes create, update, and create-or-update write semantics.
func (s *Store) save(
	bucketName []byte,
	value IDAccessor,
	validate validationFunc,
	mode saveMode,
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
			validateErr := validate(tx, value)
			if validateErr != nil {
				return validateErr
			}
		}

		bucket, bucketErr := getBucket(tx, bucketName)
		if bucketErr != nil {
			return bucketErr
		}

		existing := bucket.Get([]byte(id))
		switch mode {
		case saveModeUpdate:
			if existing == nil {
				return fmt.Errorf("%w: %s %q", ErrNotFound, string(bucketName), id)
			}
		case saveModeCreate:
			if existing != nil {
				return fmt.Errorf("%w: %s %q", ErrAlreadyExists, string(bucketName), id)
			}
		case saveModeCreateOrUpdate:
			// no-op
		default:
			return fmt.Errorf("unknown save mode: %d", mode)
		}

		mustExist := existing != nil

		timestampErr := applyTimestamps(value, mustExist, existing, time.Now().UTC())
		if timestampErr != nil {
			return fmt.Errorf("failed to set timestamps: %w", timestampErr)
		}

		payload, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal %s %q: %w", string(bucketName), id, err)
		}

		action := "create-or-update"
		switch mode {
		case saveModeCreate:
			action = "create"
		case saveModeUpdate:
			action = "update"
		case saveModeCreateOrUpdate:
			action = "create-or-update"
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

func (s *Store) create(bucketName []byte, value IDAccessor, validate validationFunc) error {
	return s.save(bucketName, value, validate, saveModeCreate)
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

func (s *Store) update(bucketName []byte, value IDAccessor, validate validationFunc) error {
	return s.save(bucketName, value, validate, saveModeUpdate)
}

func (s *Store) createOrUpdate(bucketName []byte, value IDAccessor, validate validationFunc) error {
	return s.save(bucketName, value, validate, saveModeCreateOrUpdate)
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

func getParametersForType(value any) (*typeStoreParameters, error) {
	valueType := reflect.TypeOf(value)
	parameters, ok := typeParameters[valueType]
	if !ok {
		return nil, fmt.Errorf("unsupported type: %s", valueType.String())
	}
	return &parameters, nil
}

func getBucketNameForType(value any) ([]byte, error) {
	valueType := reflect.TypeOf(value)
	parameters, ok := typeParameters[valueType]
	if !ok {
		return nil, fmt.Errorf("unsupported type: %s", valueType.String())
	}
	return parameters.bucketName, nil
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

// Same as before but with type passed as parameter, using reflect.
func (s *Store) ListItems(out any) error {
	if out == nil {
		return fmt.Errorf("output parameter is required")
	}
	outValue := reflect.ValueOf(out)
	if outValue.Kind() != reflect.Pointer || outValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("output parameter must be a pointer to a slice")
	}
	sliceValue := outValue.Elem()
	// Get the element type of the slice
	itemType := sliceValue.Type().Elem()

	parameters, ok := typeParameters[reflect.PointerTo(itemType)]
	if !ok {
		return fmt.Errorf("unsupported type: %s", itemType.String())
	}
	bucketName := parameters.bucketName
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketName)
		if err != nil {
			return err
		}
		return bucket.ForEach(func(_, value []byte) error {
			itemPtr := reflect.New(itemType)
			if err = json.Unmarshal(value, itemPtr.Interface()); err != nil {
				return fmt.Errorf("failed to decode %s record: %w", string(bucketName), err)
			}
			sliceValue.Set(reflect.Append(sliceValue, itemPtr.Elem()))
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", string(bucketName), err)
	}
	return nil
}

func CreateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	parameters, err := getParametersForType(item)
	if err != nil {
		return err
	}
	return s.create(parameters.bucketName, item, parameters.validate)
}

func DeleteItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	bucketName, err := getBucketNameForType(item)
	if err != nil {
		return err
	}
	return s.delete(bucketName, item.GetID())
}

func UpdateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	bucketName, err := getBucketNameForType(item)
	if err != nil {
		return err
	}
	return s.update(bucketName, item, nil)
}

func CreateOrUpdateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	bucketName, err := getBucketNameForType(item)
	if err != nil {
		return err
	}
	parameters, err := getParametersForType(item)
	if err != nil {
		return err
	}
	return s.createOrUpdate(bucketName, item, parameters.validate)
}

func GetItem[T any](s *Store, id string) (*T, error) {
	var item T
	bucketName, err := getBucketNameForType(&item)
	if err != nil {
		return nil, err
	}
	if err := s.get(bucketName, id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func ListItems[T any](s *Store) ([]T, error) {
	var item T
	bucketName, err := getBucketNameForType(&item)
	if err != nil {
		return nil, err
	}
	return list[T](s, bucketName)
}

func (s *Store) CreateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	parameters, err := getParametersForType(item)
	if err != nil {
		return err
	}
	return s.create(parameters.bucketName, item, parameters.validate)
}

func (s *Store) UpdateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	parameters, err := getParametersForType(item)
	if err != nil {
		return err
	}
	return s.update(parameters.bucketName, item, parameters.validate)
}

func (s *Store) CreateOrUpdateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	parameters, err := getParametersForType(item)
	if err != nil {
		return err
	}
	return s.createOrUpdate(parameters.bucketName, item, parameters.validate)
}

func (s *Store) DeleteItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	bucketName, err := getBucketNameForType(item)
	if err != nil {
		return err
	}
	return s.delete(bucketName, item.GetID())
}

func (s *Store) GetItem(id string, out any) error {
	if out == nil {
		return fmt.Errorf("output parameter is required")
	}
	bucketName, err := getBucketNameForType(out)
	if err != nil {
		return err
	}
	return s.get(bucketName, id, out)
}

// SetNameRef stores a mapping from an image name to its reference key in the images bucket.
func (s *Store) SetNameRef(name, ref string) error {
	if name == "" {
		return fmt.Errorf("image name is required")
	}
	if ref == "" {
		return fmt.Errorf("image reference is required")
	}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketImageNameRefs)
		if err != nil {
			return err
		}
		if err = bucket.Put([]byte(name), []byte(ref)); err != nil {
			return fmt.Errorf("failed to store name reference %q: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to set name reference %q: %w", name, err)
	}
	return nil
}

// GetNameRef retrieves the image reference key for the given image name.
func (s *Store) GetNameRef(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("image name is required")
	}

	var ref string
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket, err := getBucket(tx, bucketImageNameRefs)
		if err != nil {
			return err
		}
		value := bucket.Get([]byte(name))
		if value == nil {
			return fmt.Errorf("%w: image name %q", ErrNotFound, name)
		}
		ref = string(value)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to get name reference %q: %w", name, err)
	}
	return ref, nil
}
