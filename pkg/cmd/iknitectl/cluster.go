package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// CreateClusterCmd creates the cluster command tree.
func CreateClusterCmd(_ *dig.Scope) *cobra.Command {
	clusterCmd := &cobra.Command{
		Use:     "cluster",
		Aliases: []string{"c", "cl"},
		Short:   "Manage iknite clusters",
	}

	return clusterCmd
}
