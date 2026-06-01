// cSpell: words oras wrapcheck
package image

import (
	"encoding/json"
	"fmt"
	"time"
)

type ImageType int

const (
	ImageTypeUnknown ImageType = iota
	ImageTypeRootFS
	ImageTypeVHDX
	ImageTypeQCOW2
)

const (
	UnknownLabel   = "unknown"
	RootFSLabel    = "rootfs"
	VHDXLabel      = "vm-vhdx"
	QCOW2Label     = "vm-qcow2"
	IncusMetaLabel = "incus-metadata"
)

func (it ImageType) String() string {
	switch it {
	case ImageTypeRootFS:
		return RootFSLabel
	case ImageTypeVHDX:
		return VHDXLabel
	case ImageTypeQCOW2:
		return QCOW2Label
	default:
		return UnknownLabel
	}
}

func (it ImageType) FileExtension() string {
	switch it {
	case ImageTypeRootFS:
		return "tar.gz"
	case ImageTypeVHDX:
		return "vhdx"
	case ImageTypeQCOW2:
		return "qcow2"
	default:
		return "bin"
	}
}

func ParseImageType(s string) (ImageType, error) {
	switch s {
	case RootFSLabel:
		return ImageTypeRootFS, nil
	case VHDXLabel:
		return ImageTypeVHDX, nil
	case QCOW2Label:
		return ImageTypeQCOW2, nil
	default:
		return 0, fmt.Errorf("invalid image type: %s", s)
	}
}

func (it ImageType) OrasMediaType() string {
	switch it {
	case ImageTypeRootFS:
		return "application/vnd.oci.image.layer.v1.tar+gzip"
	case ImageTypeVHDX:
		return "application/vnd.openstack.image"
	case ImageTypeQCOW2:
		return "application/vnd.openstack.image"
	default:
		return "application/octet-stream"
	}
}

func (it ImageType) MarshalJSON() ([]byte, error) {
	return json.Marshal(it.String()) //nolint:wrapcheck // Simple marshal to string.
}

func (it *ImageType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err //nolint:wrapcheck // Simple unmarshal from string.
	}
	parsed, err := ParseImageType(s)
	if err != nil {
		return err
	}
	*it = parsed
	return nil
}

type ArtifactType int

const (
	ArtifactTypeUnknown ArtifactType = iota
	ArtifactTypeRootFS
	ArtifactTypeVHDXImage
	ArtifactTypeQCOW2Image
	ArtifactTypeIncusMetadata
)

func (at ArtifactType) String() string {
	switch at {
	case ArtifactTypeRootFS:
		return RootFSLabel
	case ArtifactTypeVHDXImage:
		return VHDXLabel
	case ArtifactTypeQCOW2Image:
		return QCOW2Label
	case ArtifactTypeIncusMetadata:
		return IncusMetaLabel
	default:
		return UnknownLabel
	}
}

func ParseArtifactType(s string) (ArtifactType, error) {
	switch s {
	case RootFSLabel:
		return ArtifactTypeRootFS, nil
	case VHDXLabel:
		return ArtifactTypeVHDXImage, nil
	case QCOW2Label:
		return ArtifactTypeQCOW2Image, nil
	case IncusMetaLabel:
		return ArtifactTypeIncusMetadata, nil
	default:
		return 0, fmt.Errorf("invalid artifact type: %s", s)
	}
}

func (at ArtifactType) MarshalJSON() ([]byte, error) {
	return json.Marshal(at.String()) //nolint:wrapcheck // Simple marshal to string.
}

func (at *ArtifactType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err //nolint:wrapcheck // Simple unmarshal from string.
	}
	parsed, err := ParseArtifactType(s)
	if err != nil {
		return err
	}
	*at = parsed
	return nil
}

// ImageListItem describes one image entry shown by the ls command.
type ImageListItem struct {
	UpdatedAt time.Time
	Name      string
	Source    string
	Reference string
	Path      string
	Artifacts string
	TotalSize int64
}
