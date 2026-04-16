// cSpell: words wrapcheck
package host

import "syscall"

type System interface {
	Unmount(path string) error
}

var _ System = (*hostImpl)(nil)

func NewDefaultSystemHost() System {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}

func (s *hostImpl) Unmount(path string) error {
	return syscall.Unmount(path, 0) //nolint:wrapcheck // preserve the original error type.
}
