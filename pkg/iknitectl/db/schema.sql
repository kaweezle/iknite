CREATE TABLE IF NOT EXISTS image_sources (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    kind TEXT NOT NULL,
    location TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS image_versions (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    source_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    manifest_digest TEXT NOT NULL DEFAULT '',
    manifest_media_type TEXT NOT NULL DEFAULT '',
    manifest BLOB,
    FOREIGN KEY (source_id) REFERENCES image_sources(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS images (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    version_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (version_id) REFERENCES image_versions(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS image_artifacts (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    image_id TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    digest TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (image_id) REFERENCES images(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS backend_images (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    backend TEXT NOT NULL,
    image_id TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    placeholder INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (image_id) REFERENCES images(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS clusters (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    name TEXT NOT NULL,
    backend TEXT NOT NULL,
    image_id TEXT NOT NULL,
    backend_image_id TEXT NOT NULL,
    workspace TEXT NOT NULL DEFAULT '',
    ref TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (image_id) REFERENCES images(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    FOREIGN KEY (backend_image_id) REFERENCES backend_images(id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
