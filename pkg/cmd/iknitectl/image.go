package iknitectl

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageCmd creates the image command tree.
func CreateImageCmd(s *dig.Scope) *cobra.Command {
	cobra.CheckErr(s.Provide(func(store *db.Store) image.MetadataStore { return store }))
	cobra.CheckErr(s.Provide(image.NewService))

	imageCmd := &cobra.Command{
		Use:     "image",
		Aliases: []string{"i", "img"},
		Short:   "Manage provisioning images",
	}

	imageCmd.AddCommand(CreateImageInfoCmd(s))
	imageCmd.AddCommand(CreateImageListCmd(s))
	imageCmd.AddCommand(CreateImageInspectCmd(s))
	imageCmd.AddCommand(CreateImagePullCmd(s))
	imageCmd.AddCommand(CreateImageRemoveCmd(s))

	return imageCmd
}
