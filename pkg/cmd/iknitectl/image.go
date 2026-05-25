package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
)

// CreateImageCmd creates the image command tree.
func CreateImageCmd(localHost host.Host) *cobra.Command {
	imageCmd := &cobra.Command{
		Use:     "image",
		Aliases: []string{"i", "img"},
		Short:   "Manage provisioning images",
	}

	imageCmd.AddCommand(CreateImageInspectCmd(localHost))
	imageCmd.AddCommand(CreateImagePullCmd(localHost))

	return imageCmd
}
