package iknitectl

import "github.com/spf13/cobra"

// CreateEnvCmd creates the env command tree.
func CreateEnvCmd(deps *RootDependencies) *cobra.Command {
	envCmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"e"},
		Short:   "Manage iknitectl local environment",
	}

	envCmd.AddCommand(CreateEnvInitCmd(deps))

	return envCmd
}
