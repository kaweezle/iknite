package iknitectl

import (
	"github.com/kaweezle/iknite/pkg/host"
)

// RootDependencies groups runtime dependencies for command handlers.
type RootDependencies struct {
	Host host.Host
}

// NewRootDependencies creates default runtime dependencies.
func NewRootDependencies() *RootDependencies {
	return &RootDependencies{
		Host: host.NewDefaultHost(),
	}
}
