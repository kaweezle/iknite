package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
)

// CreateEnvCmd creates the env command tree.
func CreateEnvCmd(localHost host.Host) *cobra.Command {
	envCmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"e"},
		Short:   "Manage iknitectl local environment",
	}

	envCmd.AddCommand(CreateEnvInitCmd(localHost))

	return envCmd
}
