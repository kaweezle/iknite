package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// CreateAuthCmd creates the auth command tree.
func CreateAuthCmd(s *dig.Scope) *cobra.Command {
	authCmd := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"a"},
		Short:   "Manage credentials and key material",
	}

	authCmd.AddCommand(CreateSigningKeyCmd(s, nil))

	return authCmd
}
