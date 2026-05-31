-- name: CreateImageSource :exec
INSERT INTO image_sources (id, created_at, updated_at, kind, location)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(kind), sqlc.arg(location));

-- name: UpdateImageSource :exec
UPDATE image_sources
SET updated_at = sqlc.arg(updated_at), kind = sqlc.arg(kind), location = sqlc.arg(location)
WHERE id = sqlc.arg(id);

-- name: UpsertImageSource :exec
INSERT INTO image_sources (id, created_at, updated_at, kind, location)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(kind), sqlc.arg(location))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, kind = excluded.kind, location = excluded.location;

-- name: GetImageSource :one
SELECT id, created_at, updated_at, kind, location FROM image_sources WHERE id = sqlc.arg(id);

-- name: ListImageSources :many
SELECT id, created_at, updated_at, kind, location FROM image_sources ORDER BY id;

-- name: DeleteImageSource :exec
DELETE FROM image_sources WHERE id = sqlc.arg(id);

-- name: CreateImageVersion :exec
INSERT INTO image_versions (id, created_at, updated_at, source_id, tag, manifest_digest, manifest_media_type, manifest)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(source_id), sqlc.arg(tag), sqlc.arg(manifest_digest), sqlc.arg(manifest_media_type), sqlc.arg(manifest));

-- name: UpdateImageVersion :exec
UPDATE image_versions
SET updated_at = sqlc.arg(updated_at), source_id = sqlc.arg(source_id), tag = sqlc.arg(tag), manifest_digest = sqlc.arg(manifest_digest), manifest_media_type = sqlc.arg(manifest_media_type), manifest = sqlc.arg(manifest)
WHERE id = sqlc.arg(id);

-- name: UpsertImageVersion :exec
INSERT INTO image_versions (id, created_at, updated_at, source_id, tag, manifest_digest, manifest_media_type, manifest)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(source_id), sqlc.arg(tag), sqlc.arg(manifest_digest), sqlc.arg(manifest_media_type), sqlc.arg(manifest))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, source_id = excluded.source_id, tag = excluded.tag, manifest_digest = excluded.manifest_digest, manifest_media_type = excluded.manifest_media_type, manifest = excluded.manifest;

-- name: GetImageVersion :one
SELECT id, created_at, updated_at, source_id, tag, manifest_digest, manifest_media_type, manifest FROM image_versions WHERE id = sqlc.arg(id);

-- name: ListImageVersions :many
SELECT id, created_at, updated_at, source_id, tag, manifest_digest, manifest_media_type, manifest FROM image_versions ORDER BY id;

-- name: DeleteImageVersion :exec
DELETE FROM image_versions WHERE id = sqlc.arg(id);

-- name: CreateImage :exec
INSERT INTO images (id, created_at, updated_at, version_id, name)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(version_id), sqlc.arg(name));

-- name: UpdateImage :exec
UPDATE images SET updated_at = sqlc.arg(updated_at), version_id = sqlc.arg(version_id), name = sqlc.arg(name) WHERE id = sqlc.arg(id);

-- name: UpsertImage :exec
INSERT INTO images (id, created_at, updated_at, version_id, name)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(version_id), sqlc.arg(name))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, version_id = excluded.version_id, name = excluded.name;

-- name: GetImage :one
SELECT id, created_at, updated_at, version_id, name FROM images WHERE id = sqlc.arg(id);

-- name: ListImages :many
SELECT id, created_at, updated_at, version_id, name FROM images ORDER BY id;

-- name: DeleteImage :exec
DELETE FROM images WHERE id = sqlc.arg(id);

-- name: CreateImageArtifact :exec
INSERT INTO image_artifacts (id, created_at, updated_at, image_id, path, digest, type, size)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(image_id), sqlc.arg(path), sqlc.arg(digest), sqlc.arg(type), sqlc.arg(size));

-- name: UpdateImageArtifact :exec
UPDATE image_artifacts SET updated_at = sqlc.arg(updated_at), image_id = sqlc.arg(image_id), path = sqlc.arg(path), digest = sqlc.arg(digest), type = sqlc.arg(type), size = sqlc.arg(size) WHERE id = sqlc.arg(id);

-- name: UpsertImageArtifact :exec
INSERT INTO image_artifacts (id, created_at, updated_at, image_id, path, digest, type, size)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(image_id), sqlc.arg(path), sqlc.arg(digest), sqlc.arg(type), sqlc.arg(size))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, image_id = excluded.image_id, path = excluded.path, digest = excluded.digest, type = excluded.type, size = excluded.size;

-- name: GetImageArtifact :one
SELECT id, created_at, updated_at, image_id, path, digest, type, size FROM image_artifacts WHERE id = sqlc.arg(id);

-- name: ListImageArtifacts :many
SELECT id, created_at, updated_at, image_id, path, digest, type, size FROM image_artifacts ORDER BY id;

-- name: DeleteImageArtifact :exec
DELETE FROM image_artifacts WHERE id = sqlc.arg(id);

-- name: CreateBackendImage :exec
INSERT INTO backend_images (id, created_at, updated_at, backend, image_id, external_id, placeholder)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(backend), sqlc.arg(image_id), sqlc.arg(external_id), sqlc.arg(placeholder));

-- name: UpdateBackendImage :exec
UPDATE backend_images SET updated_at = sqlc.arg(updated_at), backend = sqlc.arg(backend), image_id = sqlc.arg(image_id), external_id = sqlc.arg(external_id), placeholder = sqlc.arg(placeholder) WHERE id = sqlc.arg(id);

-- name: UpsertBackendImage :exec
INSERT INTO backend_images (id, created_at, updated_at, backend, image_id, external_id, placeholder)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(backend), sqlc.arg(image_id), sqlc.arg(external_id), sqlc.arg(placeholder))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, backend = excluded.backend, image_id = excluded.image_id, external_id = excluded.external_id, placeholder = excluded.placeholder;

-- name: GetBackendImage :one
SELECT id, created_at, updated_at, backend, image_id, external_id, placeholder FROM backend_images WHERE id = sqlc.arg(id);

-- name: ListBackendImages :many
SELECT id, created_at, updated_at, backend, image_id, external_id, placeholder FROM backend_images ORDER BY id;

-- name: DeleteBackendImage :exec
DELETE FROM backend_images WHERE id = sqlc.arg(id);

-- name: CreateCluster :exec
INSERT INTO clusters (id, created_at, updated_at, name, backend, image_id, backend_image_id, workspace, ref)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(name), sqlc.arg(backend), sqlc.arg(image_id), sqlc.arg(backend_image_id), sqlc.arg(workspace), sqlc.arg(ref));

-- name: UpdateCluster :exec
UPDATE clusters SET updated_at = sqlc.arg(updated_at), name = sqlc.arg(name), backend = sqlc.arg(backend), image_id = sqlc.arg(image_id), backend_image_id = sqlc.arg(backend_image_id), workspace = sqlc.arg(workspace), ref = sqlc.arg(ref) WHERE id = sqlc.arg(id);

-- name: UpsertCluster :exec
INSERT INTO clusters (id, created_at, updated_at, name, backend, image_id, backend_image_id, workspace, ref)
VALUES (sqlc.arg(id), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(name), sqlc.arg(backend), sqlc.arg(image_id), sqlc.arg(backend_image_id), sqlc.arg(workspace), sqlc.arg(ref))
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, name = excluded.name, backend = excluded.backend, image_id = excluded.image_id, backend_image_id = excluded.backend_image_id, workspace = excluded.workspace, ref = excluded.ref;

-- name: GetCluster :one
SELECT id, created_at, updated_at, name, backend, image_id, backend_image_id, workspace, ref FROM clusters WHERE id = sqlc.arg(id);

-- name: ListClusters :many
SELECT id, created_at, updated_at, name, backend, image_id, backend_image_id, workspace, ref FROM clusters ORDER BY id;

-- name: DeleteCluster :exec
DELETE FROM clusters WHERE id = sqlc.arg(id);
