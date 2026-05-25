package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
)

// CreateAuthCmd creates the auth command tree.
func CreateAuthCmd(localHost host.Host) *cobra.Command {
	authCmd := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"a"},
		Short:   "Manage credentials and key material",
	}

	authCmd.AddCommand(CreateSigningKeyCmd(localHost, nil))

	return authCmd
}
