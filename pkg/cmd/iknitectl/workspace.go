package iknitectl

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/cmd/secrets"
	"github.com/kaweezle/iknite/pkg/host"
)

// CreateWorkspaceCmd creates the workspace command tree.
func CreateWorkspaceCmd(fileExecutor host.FileExecutor, out io.Writer) *cobra.Command {
	workspaceCmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"w", "ws"},
		Short:   "Manage workspace-level operations",
	}

	workspaceCmd.AddCommand(CreateApplicationCmd(fileExecutor, out))
	workspaceCmd.AddCommand(CreateKustomizeCmd(fileExecutor, out))
	workspaceCmd.AddCommand(secrets.CreateSecretsCmd(fileExecutor, nil))

	return workspaceCmd
}
