package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
)

// CreateBackendCmd creates the backend command tree.
func CreateBackendCmd(_ host.Host) *cobra.Command {
	backendCmd := &cobra.Command{
		Use:     "backend",
		Aliases: []string{"b", "bck"},
		Short:   "Manage backend definitions",
	}

	return backendCmd
}
