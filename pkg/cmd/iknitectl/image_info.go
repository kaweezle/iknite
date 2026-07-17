// cSpell: words imagesvc
package iknitectl

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

type ImageNameRequest string

// CreateImageInfoCmd creates the image info command.
func CreateImageInfoCmd(s *dig.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <image-name>",
		Short: "Show extended information about a downloaded image",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			err := s.Provide(func() ImageNameRequest { return ImageNameRequest(args[0]) })
			if err != nil {
				return fmt.Errorf("failed to provide image name: %w", err)
			}
			return s.Invoke(performImageInfo)
		},
	}

	return cmd
}

func performImageInfo(service *imagesvc.Service, imageName ImageNameRequest, out types.CmdOut) error {
	result, err := service.Info(string(imageName))
	if err != nil {
		return fmt.Errorf("failed to get image info: %w", err)
	}
	service.Logger.Info("Image info retrieved",
		"name", result.Name,
		"source", result.Source.Location,
		"reference", result.Reference,
		"artifacts", len(result.Artifacts),
	)

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal image info: %w", err)
	}
	if _, err = fmt.Fprintln(out, string(output)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
