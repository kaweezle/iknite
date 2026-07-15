package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/secrets"
)

// CreateWorkspaceCmd creates the workspace command tree.
func CreateWorkspaceCmd(s *dig.Scope) *cobra.Command {
	workspaceCmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"w", "ws"},
		Short:   "Manage workspace-level operations",
	}

	workspaceCmd.AddCommand(CreateApplicationCmd(s))
	workspaceCmd.AddCommand(secrets.CreateSecretsCmd(s, nil))

	return workspaceCmd
}
