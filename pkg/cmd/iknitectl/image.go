package iknitectl

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
)

// CreateImageCmd creates the image command tree.
func CreateImageCmd(baseService base.ServiceInterface) *cobra.Command {
	imageCmd := &cobra.Command{
		Use:     "image",
		Aliases: []string{"i", "img"},
		Short:   "Manage provisioning images",
	}

	imageCmd.AddCommand(CreateImageInfoCmd(baseService))
	imageCmd.AddCommand(CreateImageListCmd(baseService))
	imageCmd.AddCommand(CreateImageInspectCmd(baseService))
	imageCmd.AddCommand(CreateImagePullCmd(baseService))
	imageCmd.AddCommand(CreateImageRemoveCmd(baseService))

	return imageCmd
}
