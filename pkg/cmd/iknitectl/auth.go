package iknitectl

import "github.com/spf13/cobra"

// CreateAuthCmd creates the auth command tree.
func CreateAuthCmd(deps *RootDependencies) *cobra.Command {
	authCmd := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"a"},
		Short:   "Manage credentials and key material",
	}

	authCmd.AddCommand(CreateSigningKeyCmd(deps.Host, nil))

	return authCmd
}
