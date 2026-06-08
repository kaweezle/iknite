package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// CreateEnvCmd creates the env command tree.
func CreateEnvCmd(s *dig.Scope) *cobra.Command {
	envCmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"e"},
		Short:   "Manage iknitectl local environment",
	}

	envCmd.AddCommand(CreateEnvInitCmd(s.Scope("init")))

	return envCmd
}
