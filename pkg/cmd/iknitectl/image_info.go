// cSpell: words imagesvc
package iknitectl

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageInfoCmd creates the image info command.
func CreateImageInfoCmd(baseService base.ServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <image-name>",
		Short: "Show extended information about a downloaded image",
		Args:  cobra.ExactArgs(1),
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
			result, err := service.Info(args[0])
			if err != nil {
				return fmt.Errorf("failed to get image info: %w", err)
			}

			logger := baseService.Logger()
			logger.Info("Image info retrieved",
				"name", result.Name,
				"source", result.Source.Location,
				"reference", result.Reference,
				"artifacts", len(result.Artifacts),
			)

			output, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal image info: %w", err)
			}
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), string(output)); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
	}

	return cmd
}
