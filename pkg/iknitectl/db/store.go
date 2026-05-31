// cSpell: words sqlc
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"

	"github.com/kaweezle/iknite/pkg/iknitectl/db/sqlc"
)

var (
	// ErrNotFound is returned when a record cannot be found.
	ErrNotFound = errors.New("record not found")
	// ErrAlreadyExists is returned when creating a record with an existing ID.
	ErrAlreadyExists = errors.New("record already exists")
	// ErrInvalidID is returned when a record ID is empty.
	ErrInvalidID = errors.New("invalid id")
)

//go:embed schema.sql
var schemaFS embed.FS

type saveMode int

const (
	saveModeCreate saveMode = iota
	saveModeUpdate
	saveModeCreateOrUpdate
)

// timestampFormat keeps database timestamps human-readable while preserving nanosecond precision.
const timestampFormat = time.RFC3339Nano

type IDAccessorPointer[M any] interface {
	*M
	IDAccessor
}

// Store provides SQLite-backed persistence for iknitectl client objects.
type Store struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// Open opens the database and initializes all required tables.
func Open(path string) (*Store, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	// Keep one connection so connection-local PRAGMA settings apply to every query and writes remain serialized.
	database.SetMaxOpenConns(1)

	store := &Store{db: database, queries: sqlc.New(database)}
	if err = store.initialize(); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize database: %w", errors.Join(err, closeErr))
		}
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return store, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close() //nolint:wrapcheck // Preserve database close error.
}

func (s *Store) initialize() error {
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL journal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}
	if _, err = s.db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	return nil
}

func requireID(id string) error {
	if id == "" {
		return ErrInvalidID
	}
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

func timestampString(value time.Time) string {
	return value.UTC().Format(timestampFormat)
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(timestampFormat, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp %q: %w", value, err)
	}
	return parsed, nil
}

func setTimestamps(value IDAccessor, existingCreatedAt time.Time, now time.Time) error {
	accessor, ok := value.(TimestampAccessor)
	if !ok {
		return nil
	}
	if existingCreatedAt.IsZero() {
		existingCreatedAt = now
	}
	accessor.SetCreatedAt(existingCreatedAt)
	accessor.SetUpdatedAt(now)
	return nil
}

func isRowNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func normalizeSQLError(err error) error {
	if err == nil {
		return nil
	}
	var sqliteErr interface{ Code() int }
	if !errors.As(err, &sqliteErr) {
		return err
	}

	switch sqliteErr.Code() {
	case sqlitelib.SQLITE_CONSTRAINT_FOREIGNKEY:
		return fmt.Errorf("%w: referenced record not found", ErrNotFound)
	case sqlitelib.SQLITE_CONSTRAINT_PRIMARYKEY, sqlitelib.SQLITE_CONSTRAINT_UNIQUE:
		return fmt.Errorf("%w: duplicate id", ErrAlreadyExists)
	default:
		return err
	}
}

func withTx[T any](s *Store, fn func(*sqlc.Queries) (T, error)) (result T, err error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to rollback transaction: %w", rollbackErr))
			}
		}
	}()

	result, err = fn(s.queries.WithTx(tx))
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return result, nil
}

func withTxNoResult(s *Store, fn func(*sqlc.Queries) error) error {
	_, err := withTx(s, func(queries *sqlc.Queries) (struct{}, error) {
		return struct{}{}, fn(queries)
	})
	return err
}

func (s *Store) save(value IDAccessor, mode saveMode) error {
	id, err := ensureRecordID(value)
	if err != nil {
		return err
	}
	if err = requireID(id); err != nil {
		return err
	}

	err = withTxNoResult(s, func(q *sqlc.Queries) error {
		existingCreatedAt, exists, err := s.getCreatedAt(q, value)
		if err != nil {
			return err
		}
		switch mode {
		case saveModeCreate:
			if exists {
				return fmt.Errorf("%w: %q", ErrAlreadyExists, id)
			}
		case saveModeUpdate:
			if !exists {
				return fmt.Errorf("%w: %q", ErrNotFound, id)
			}
		case saveModeCreateOrUpdate:
		// no-op
		default:
			return fmt.Errorf("unknown save mode: %d", mode)
		}

		now := time.Now().UTC()
		if err = setTimestamps(value, existingCreatedAt, now); err != nil {
			return fmt.Errorf("failed to set timestamps: %w", err)
		}
		return normalizeSQLError(s.saveWithQueries(q, value, mode))
	})
	if err != nil {
		return fmt.Errorf("failed to persist %q: %w", id, err)
	}
	return nil
}

func (s *Store) getCreatedAt(q *sqlc.Queries, value IDAccessor) (time.Time, bool, error) {
	ctx := context.Background()
	switch item := value.(type) {
	case *ImageSource:
		row, err := q.GetImageSource(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	case *ImageVersion:
		row, err := q.GetImageVersion(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	case *Image:
		row, err := q.GetImage(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	case *ImageArtifact:
		row, err := q.GetImageArtifact(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	case *BackendImage:
		row, err := q.GetBackendImage(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	case *Cluster:
		row, err := q.GetCluster(ctx, item.ID)
		return getExistingTimestamp(row.CreatedAt, err)
	default:
		return time.Time{}, false, fmt.Errorf("unsupported type: %T", value)
	}
}

func getExistingTimestamp(value string, err error) (time.Time, bool, error) {
	if isRowNotFound(err) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, normalizeSQLError(err)
	}
	parsed, err := parseTimestamp(value)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}

func (s *Store) saveWithQueries(q *sqlc.Queries, value IDAccessor, mode saveMode) error { //nolint:gocyclo // Type dispatch keeps public API stable.
	ctx := context.Background()
	switch item := value.(type) {
	case *ImageSource:
		params := sqlc.CreateImageSourceParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), Kind: item.Kind, Location: item.Location}
		switch mode {
		case saveModeCreate:
			return q.CreateImageSource(ctx, params)
		case saveModeUpdate:
			return q.UpdateImageSource(ctx, sqlc.UpdateImageSourceParams{ID: params.ID, UpdatedAt: params.UpdatedAt, Kind: params.Kind, Location: params.Location})
		case saveModeCreateOrUpdate:
			return q.UpsertImageSource(ctx, sqlc.UpsertImageSourceParams(params))
		}
	case *ImageVersion:
		params := sqlc.CreateImageVersionParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), SourceID: item.SourceID, Tag: item.Tag, ManifestDigest: item.ManifestDigest, ManifestMediaType: item.ManifestMediaType, Manifest: item.Manifest}
		switch mode {
		case saveModeCreate:
			return q.CreateImageVersion(ctx, params)
		case saveModeUpdate:
			return q.UpdateImageVersion(ctx, sqlc.UpdateImageVersionParams{ID: params.ID, UpdatedAt: params.UpdatedAt, SourceID: params.SourceID, Tag: params.Tag, ManifestDigest: params.ManifestDigest, ManifestMediaType: params.ManifestMediaType, Manifest: params.Manifest})
		case saveModeCreateOrUpdate:
			return q.UpsertImageVersion(ctx, sqlc.UpsertImageVersionParams(params))
		}
	case *Image:
		params := sqlc.CreateImageParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), VersionID: item.VersionID, Name: item.Name}
		switch mode {
		case saveModeCreate:
			return q.CreateImage(ctx, params)
		case saveModeUpdate:
			return q.UpdateImage(ctx, sqlc.UpdateImageParams{ID: params.ID, UpdatedAt: params.UpdatedAt, VersionID: params.VersionID, Name: params.Name})
		case saveModeCreateOrUpdate:
			return q.UpsertImage(ctx, sqlc.UpsertImageParams(params))
		}
	case *ImageArtifact:
		params := sqlc.CreateImageArtifactParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), ImageID: item.ImageID, Path: item.Path, Digest: item.Digest, Type: string(item.Type), Size: item.Size}
		switch mode {
		case saveModeCreate:
			return q.CreateImageArtifact(ctx, params)
		case saveModeUpdate:
			return q.UpdateImageArtifact(ctx, sqlc.UpdateImageArtifactParams{ID: params.ID, UpdatedAt: params.UpdatedAt, ImageID: params.ImageID, Path: params.Path, Digest: params.Digest, Type: params.Type, Size: params.Size})
		case saveModeCreateOrUpdate:
			return q.UpsertImageArtifact(ctx, sqlc.UpsertImageArtifactParams(params))
		}
	case *BackendImage:
		placeholder := int64(0)
		if item.Placeholder {
			placeholder = 1
		}
		params := sqlc.CreateBackendImageParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), Backend: item.Backend, ImageID: item.ImageID, ExternalID: item.ExternalID, Placeholder: placeholder}
		switch mode {
		case saveModeCreate:
			return q.CreateBackendImage(ctx, params)
		case saveModeUpdate:
			return q.UpdateBackendImage(ctx, sqlc.UpdateBackendImageParams{ID: params.ID, UpdatedAt: params.UpdatedAt, Backend: params.Backend, ImageID: params.ImageID, ExternalID: params.ExternalID, Placeholder: params.Placeholder})
		case saveModeCreateOrUpdate:
			return q.UpsertBackendImage(ctx, sqlc.UpsertBackendImageParams(params))
		}
	case *Cluster:
		params := sqlc.CreateClusterParams{ID: item.ID, CreatedAt: timestampString(item.CreatedAt), UpdatedAt: timestampString(item.UpdatedAt), Name: item.Name, Backend: item.Backend, ImageID: item.ImageID, BackendImageID: item.BackendImageID, Workspace: item.Workspace, Ref: item.Ref}
		switch mode {
		case saveModeCreate:
			return q.CreateCluster(ctx, params)
		case saveModeUpdate:
			return q.UpdateCluster(ctx, sqlc.UpdateClusterParams{ID: params.ID, UpdatedAt: params.UpdatedAt, Name: params.Name, Backend: params.Backend, ImageID: params.ImageID, BackendImageID: params.BackendImageID, Workspace: params.Workspace, Ref: params.Ref})
		case saveModeCreateOrUpdate:
			return q.UpsertCluster(ctx, sqlc.UpsertClusterParams(params))
		}
	}
	return fmt.Errorf("unsupported type: %T", value)
}

func (s *Store) create(value IDAccessor) error {
	return s.save(value, saveModeCreate)
}

func (s *Store) update(value IDAccessor) error {
	return s.save(value, saveModeUpdate)
}

func (s *Store) createOrUpdate(value IDAccessor) error {
	return s.save(value, saveModeCreateOrUpdate)
}

func (s *Store) delete(value IDAccessor) error {
	if err := requireID(value.GetID()); err != nil {
		return err
	}
	err := withTxNoResult(s, func(q *sqlc.Queries) error {
		_, exists, err := s.getCreatedAt(q, value)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: %q", ErrNotFound, value.GetID())
		}
		return normalizeSQLError(s.deleteWithQueries(q, value))
	})
	if err != nil {
		return fmt.Errorf("failed to remove %q: %w", value.GetID(), err)
	}
	return nil
}

func (s *Store) deleteWithQueries(q *sqlc.Queries, value IDAccessor) error {
	ctx := context.Background()
	switch item := value.(type) {
	case *ImageSource:
		return q.DeleteImageSource(ctx, item.ID)
	case *ImageVersion:
		return q.DeleteImageVersion(ctx, item.ID)
	case *Image:
		return q.DeleteImage(ctx, item.ID)
	case *ImageArtifact:
		return q.DeleteImageArtifact(ctx, item.ID)
	case *BackendImage:
		return q.DeleteBackendImage(ctx, item.ID)
	case *Cluster:
		return q.DeleteCluster(ctx, item.ID)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
}

func (s *Store) get(id string, out any) error {
	if err := requireID(id); err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("output parameter is required")
	}
	err := s.getWithQueries(s.queries, id, out)
	if isRowNotFound(err) {
		return fmt.Errorf("failed to read %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", id, normalizeSQLError(err))
	}
	return nil
}

func (s *Store) getWithQueries(q *sqlc.Queries, id string, out any) error { //nolint:gocyclo // Type dispatch keeps public API stable.
	ctx := context.Background()
	switch item := out.(type) {
	case *ImageSource:
		row, err := q.GetImageSource(ctx, id)
		if err != nil {
			return err
		}
		converted, err := imageSourceFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	case *ImageVersion:
		row, err := q.GetImageVersion(ctx, id)
		if err != nil {
			return err
		}
		converted, err := imageVersionFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	case *Image:
		row, err := q.GetImage(ctx, id)
		if err != nil {
			return err
		}
		converted, err := imageFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	case *ImageArtifact:
		row, err := q.GetImageArtifact(ctx, id)
		if err != nil {
			return err
		}
		converted, err := imageArtifactFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	case *BackendImage:
		row, err := q.GetBackendImage(ctx, id)
		if err != nil {
			return err
		}
		converted, err := backendImageFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	case *Cluster:
		row, err := q.GetCluster(ctx, id)
		if err != nil {
			return err
		}
		converted, err := clusterFromRow(row)
		if err != nil {
			return err
		}
		*item = converted
	default:
		return fmt.Errorf("unsupported type: %T", out)
	}
	return nil
}

func baseModelFromRow(id, createdAt, updatedAt string) (BaseModel, error) {
	created, err := parseTimestamp(createdAt)
	if err != nil {
		return BaseModel{}, err
	}
	updated, err := parseTimestamp(updatedAt)
	if err != nil {
		return BaseModel{}, err
	}
	return BaseModel{ID: id, CreatedAt: created, UpdatedAt: updated}, nil
}

func imageSourceFromRow(row sqlc.ImageSource) (ImageSource, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return ImageSource{BaseModel: base, Kind: row.Kind, Location: row.Location}, err
}

func imageVersionFromRow(row sqlc.ImageVersion) (ImageVersion, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return ImageVersion{BaseModel: base, SourceID: row.SourceID, Tag: row.Tag, ManifestDigest: row.ManifestDigest, ManifestMediaType: row.ManifestMediaType, Manifest: row.Manifest}, err
}

func imageFromRow(row sqlc.Image) (Image, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return Image{BaseModel: base, VersionID: row.VersionID, Name: row.Name}, err
}

func imageArtifactFromRow(row sqlc.ImageArtifact) (ImageArtifact, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return ImageArtifact{BaseModel: base, ImageID: row.ImageID, Path: row.Path, Digest: row.Digest, Type: ArtifactType(row.Type), Size: row.Size}, err
}

func backendImageFromRow(row sqlc.BackendImage) (BackendImage, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return BackendImage{BaseModel: base, Backend: row.Backend, ImageID: row.ImageID, ExternalID: row.ExternalID, Placeholder: row.Placeholder != 0}, err
}

func clusterFromRow(row sqlc.Cluster) (Cluster, error) {
	base, err := baseModelFromRow(row.ID, row.CreatedAt, row.UpdatedAt)
	return Cluster{BaseModel: base, Name: row.Name, Backend: row.Backend, ImageID: row.ImageID, BackendImageID: row.BackendImageID, Workspace: row.Workspace, Ref: row.Ref}, err
}

func (s *Store) ListItems(out any) error {
	if out == nil {
		return fmt.Errorf("output parameter is required")
	}
	outValue := reflect.ValueOf(out)
	if outValue.Kind() != reflect.Pointer || outValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("output parameter must be a pointer to a slice")
	}
	sliceValue := outValue.Elem()
	itemType := sliceValue.Type().Elem()
	items, err := s.listForType(itemType)
	if err != nil {
		return err
	}
	for _, item := range items {
		sliceValue.Set(reflect.Append(sliceValue, reflect.ValueOf(item)))
	}
	return nil
}

func (s *Store) listForType(itemType reflect.Type) ([]any, error) { //nolint:gocyclo // Type dispatch keeps public API stable.
	ctx := context.Background()
	switch itemType {
	case reflect.TypeFor[ImageSource]():
		rows, err := s.queries.ListImageSources(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list image_sources: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := imageSourceFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case reflect.TypeFor[ImageVersion]():
		rows, err := s.queries.ListImageVersions(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list image_versions: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := imageVersionFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case reflect.TypeFor[Image]():
		rows, err := s.queries.ListImages(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list images: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := imageFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case reflect.TypeFor[ImageArtifact]():
		rows, err := s.queries.ListImageArtifacts(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list image_artifacts: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := imageArtifactFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case reflect.TypeFor[BackendImage]():
		rows, err := s.queries.ListBackendImages(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list backend_images: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := backendImageFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case reflect.TypeFor[Cluster]():
		rows, err := s.queries.ListClusters(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", normalizeSQLError(err))
		}
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			item, err := clusterFromRow(row)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", itemType.String())
	}
}

func CreateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.create(item)
}

func DeleteItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.delete(item)
}

func UpdateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.update(item)
}

func CreateOrUpdateItem[T any, PT IDAccessorPointer[T]](s *Store, item PT) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.createOrUpdate(item)
}

func GetItem[T any](s *Store, id string) (*T, error) {
	var item T
	if err := s.get(id, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func ListItems[T any](s *Store) ([]T, error) {
	items := make([]T, 0)
	if err := s.ListItems(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CreateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.create(item)
}

func (s *Store) UpdateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.update(item)
}

func (s *Store) CreateOrUpdateItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.createOrUpdate(item)
}

func (s *Store) DeleteItem(item IDAccessor) error {
	if item == nil {
		return fmt.Errorf("item is required")
	}
	return s.delete(item)
}

func (s *Store) GetItem(id string, out any) error {
	return s.get(id, out)
}
