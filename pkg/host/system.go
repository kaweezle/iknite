// cSpell: words wrapcheck
package host

import "syscall"

type System interface {
	Unmount(path string) error
}

type SystemHost struct{}

var _ System = (*SystemHost)(nil)

func NewDefaultSystemHost() *SystemHost {
	return &SystemHost{}
}

func (s *SystemHost) Unmount(path string) error {
	return syscall.Unmount(path, 0) //nolint:wrapcheck // preserve the original error type.
}
