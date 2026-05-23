package iknitectl

import "github.com/spf13/cobra"

// CreateClusterCmd creates the cluster command tree.
func CreateClusterCmd(_ *RootDependencies) *cobra.Command {
	clusterCmd := &cobra.Command{
		Use:     "cluster",
		Aliases: []string{"c", "cl"},
		Short:   "Manage iknite clusters",
	}

	return clusterCmd
}
