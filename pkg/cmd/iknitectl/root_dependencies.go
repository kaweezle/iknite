package iknitectl

import (
	"os"
	"runtime"

	"github.com/kaweezle/iknite/pkg/host"
)

// EnvironmentProvider provides process environment access for dependency injection.
type EnvironmentProvider interface {
	Getenv(key string) string
}

// PlatformDetector provides platform access for dependency injection.
type PlatformDetector interface {
	GOOS() string
}

// RootDependencies groups runtime dependencies for command handlers.
type RootDependencies struct {
	Host     host.Host
	Env      EnvironmentProvider
	Platform PlatformDetector
}

type osEnvironmentProvider struct{}

func (p *osEnvironmentProvider) Getenv(key string) string {
	return os.Getenv(key)
}

type runtimePlatformDetector struct{}

func (d *runtimePlatformDetector) GOOS() string {
	return runtime.GOOS
}

// NewRootDependencies creates default runtime dependencies.
func NewRootDependencies() *RootDependencies {
	return &RootDependencies{
		Host:     host.NewDefaultHost(),
		Env:      &osEnvironmentProvider{},
		Platform: &runtimePlatformDetector{},
	}
}
