// cSpell: words imagesvc
package iknitectl

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageRemoveCmd creates the image remove command.
func CreateImageRemoveCmd(s *dig.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <image-name>",
		Aliases: []string{"rm"},
		Short:   "Remove a downloaded image and its artifacts",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := s.Provide(func() ImageNameRequest { return ImageNameRequest(args[0]) }); err != nil {
				return fmt.Errorf("providing image name: %w", err)
			}
			return s.Invoke(performImageRemove)
		},
	}

	return cmd
}

func performImageRemove(service *imagesvc.Service, imageName ImageNameRequest, out types.CmdOut) error {
	if err := service.Remove(string(imageName)); err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	if _, err := fmt.Fprintf(out, "removed image %q\n", imageName); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
