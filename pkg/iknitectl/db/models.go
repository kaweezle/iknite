package db

import (
	"time"
)

// ArtifactType identifies the format of a stored image artifact.
type ArtifactType string

const (
	ArtifactTypeUnknown       ArtifactType = "unknown"
	ArtifactTypeRootFS        ArtifactType = "rootfs"
	ArtifactTypeVHDXImage     ArtifactType = "vm-vhdx"
	ArtifactTypeQCOW2Image    ArtifactType = "vm-qcow2"
	ArtifactTypeIncusMetadata ArtifactType = "incus-metadata"
)

// BaseModel contains common metadata shared by all persisted objects.
type BaseModel struct {
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	ID        string    `json:"id"`
}

// GetID returns the record identifier.
func (b *BaseModel) GetID() string {
	return b.ID
}

// SetID sets the record identifier.
func (b *BaseModel) SetID(value string) {
	b.ID = value
}

// GetCreatedAt returns the creation time.
func (b *BaseModel) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// SetCreatedAt sets the creation time.
func (b *BaseModel) SetCreatedAt(value time.Time) {
	b.CreatedAt = value
}

// GetUpdatedAt returns the update time.
func (b *BaseModel) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}

// SetUpdatedAt sets the update time.
func (b *BaseModel) SetUpdatedAt(value time.Time) {
	b.UpdatedAt = value
}

// CreatedAtAccessor provides access to a record creation timestamp.
type CreatedAtAccessor interface {
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
}

// UpdatedAtAccessor provides access to a record update timestamp.
type UpdatedAtAccessor interface {
	GetUpdatedAt() time.Time
	SetUpdatedAt(time.Time)
}

// TimestampAccessor groups creation and update timestamp accessors.
type TimestampAccessor interface {
	CreatedAtAccessor
	UpdatedAtAccessor
}

// IDAccessor provides access to a record identifier.
type IDAccessor interface {
	GetID() string
	SetID(string)
}

// ImageSource describes where images can be discovered (registry, file path, and so on).
type ImageSource struct {
	BaseModel
	Kind     string `json:"kind"`
	Location string `json:"location"`
}

// ImageVersion describes a version available from an image source.
type ImageVersion struct {
	BaseModel
	SourceID          string `json:"sourceId"`
	Tag               string `json:"tag"`
	ManifestDigest    string `json:"manifestDigest,omitempty"`
	ManifestMediaType string `json:"manifestMediaType,omitempty"`
	Manifest          []byte `json:"manifest,omitempty"`
}

// Image describes a downloaded or imported image tied to one image version.
type Image struct {
	BaseModel
	VersionID string `json:"versionId"`
	Name      string `json:"name,omitempty"`
}

// ImageArtifact describes one physical artifact associated with an image.
type ImageArtifact struct {
	BaseModel
	ImageID string       `json:"imageId"`
	Path    string       `json:"path,omitempty"`
	Digest  string       `json:"digest,omitempty"`
	Type    ArtifactType `json:"type"`
	Size    int64        `json:"size,omitempty"`
}

// BackendImage describes one image import into a backend provider.
type BackendImage struct {
	BaseModel
	Backend     string `json:"backend"`
	ImageID     string `json:"imageId"`
	ExternalID  string `json:"externalId,omitempty"`
	Placeholder bool   `json:"placeholder,omitempty"`
}

// Cluster describes one iknite cluster instance.
type Cluster struct {
	BaseModel
	Name           string `json:"name"`
	Backend        string `json:"backend"`
	ImageID        string `json:"imageId"`
	BackendImageID string `json:"backendImageId"`
	Workspace      string `json:"workspace,omitempty"`
	Ref            string `json:"ref,omitempty"`
}
