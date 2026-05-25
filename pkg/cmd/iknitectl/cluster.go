package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
)

// CreateClusterCmd creates the cluster command tree.
func CreateClusterCmd(_ host.Host) *cobra.Command {
	clusterCmd := &cobra.Command{
		Use:     "cluster",
		Aliases: []string{"c", "cl"},
		Short:   "Manage iknite clusters",
	}

	return clusterCmd
}
