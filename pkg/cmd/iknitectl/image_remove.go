// cSpell: words imagesvc
package iknitectl

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageRemoveCmd creates the image remove command.
func CreateImageRemoveCmd(baseService base.ServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <image-name>",
		Aliases: []string{"rm"},
		Short:   "Remove a downloaded image and its artifacts",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := baseService.Store()
			if err != nil {
				return fmt.Errorf("failed to open metadata store: %w", err)
			}

			service := &imagesvc.Service{
				FS:     baseService.Host(),
				Logger: baseService.Logger(),
				Config: baseService.Config(),
				Store:  store,
			}
			if err = service.Remove(args[0]); err != nil {
				return fmt.Errorf("failed to remove image: %w", err)
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "removed image %q\n", args[0]); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
	}

	return cmd
}
