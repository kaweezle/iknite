package kubewait

import (
	"io"
	"os"

	"github.com/kaweezle/iknite/pkg/host"
	pkgKubewait "github.com/kaweezle/iknite/pkg/kubewait"
)

// Deps groups dependencies used to build the kubewait command.
type Deps struct {
	FileExecutor host.FileExecutor
	Options      *pkgKubewait.Options
	Out          io.Writer
}

// DefaultDeps returns production defaults for kubewait.
func DefaultDeps() *Deps {
	return &Deps{
		FileExecutor: host.NewDefaultHost(),
		Options:      pkgKubewait.NewOptions(),
		Out:          os.Stdout,
	}
}

func applyDepsDefaults(deps *Deps) *Deps {
	defaults := DefaultDeps()
	if deps == nil {
		return defaults
	}
	if deps.FileExecutor == nil {
		deps.FileExecutor = defaults.FileExecutor
	}
	if deps.Options == nil {
		deps.Options = defaults.Options
	}
	if deps.Out == nil {
		deps.Out = defaults.Out
	}
	return deps
}
