// cSpell: words wrapcheck
package host

import (
	"os"
	"path/filepath"
	"runtime"
)

type Environment interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
	Setenv(key, value string) error
	UserConfigDir() (string, error)
	UserHomeDir() (string, error)
	GOOS() string
	JoinPath(elem ...string) string
}

func (c *hostImpl) Getenv(key string) string {
	return os.Getenv(key)
}

func (c *hostImpl) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (c *hostImpl) UserConfigDir() (string, error) {
	return os.UserConfigDir() //nolint:wrapcheck // intentionally return original error
}

func (c *hostImpl) UserHomeDir() (string, error) {
	return os.UserHomeDir() //nolint:wrapcheck // intentionally return original error
}

func (c *hostImpl) GOOS() string {
	return runtime.GOOS
}

func (c *hostImpl) JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

func (c *hostImpl) Setenv(key, value string) error {
	return os.Setenv(key, value) //nolint:wrapcheck // intentionally return original error
}
