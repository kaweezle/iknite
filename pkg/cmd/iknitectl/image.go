package iknitectl

import "github.com/spf13/cobra"

// CreateImageCmd creates the image command tree.
func CreateImageCmd(deps *RootDependencies) *cobra.Command {
	imageCmd := &cobra.Command{
		Use:     "image",
		Aliases: []string{"i", "img"},
		Short:   "Manage provisioning images",
	}

	imageCmd.AddCommand(CreateImageInspectCmd(deps))
	imageCmd.AddCommand(CreateImagePullCmd(deps))

	return imageCmd
}
