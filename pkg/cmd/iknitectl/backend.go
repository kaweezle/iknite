package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// CreateBackendCmd creates the backend command tree.
func CreateBackendCmd(_ *dig.Scope) *cobra.Command {
	backendCmd := &cobra.Command{
		Use:     "backend",
		Aliases: []string{"b", "bck"},
		Short:   "Manage backend definitions",
	}

	return backendCmd
}
