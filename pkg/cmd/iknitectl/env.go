package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
)

// CreateEnvCmd creates the env command tree.
func CreateEnvCmd(baseService base.ServiceInterface) *cobra.Command {
	envCmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"e"},
		Short:   "Manage iknitectl local environment",
	}

	envCmd.AddCommand(CreateEnvInitCmd(baseService))

	return envCmd
}
