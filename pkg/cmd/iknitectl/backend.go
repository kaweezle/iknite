package iknitectl

import "github.com/spf13/cobra"

// CreateBackendCmd creates the backend command tree.
func CreateBackendCmd(_ *RootDependencies) *cobra.Command {
	backendCmd := &cobra.Command{
		Use:     "backend",
		Aliases: []string{"b", "bck"},
		Short:   "Manage backend definitions",
	}

	return backendCmd
}
