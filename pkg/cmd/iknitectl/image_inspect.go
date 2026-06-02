package iknitectl

// cSpell: words imagesvc

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageInspectCmd creates the image inspect command.
func CreateImageInspectCmd(baseService base.ServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <image-ref>",
		Short: "Inspect image manifest details",
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
			result, err := service.Inspect(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to inspect image: %w", err)
			}
			logger := baseService.Logger()
			logger.Info("Image inspection successful",
				"repository", result.Repository,
				"reference", result.Reference,
				"digest", result.Descriptor.Digest.String(),
				"layers", len(result.Manifest.Layers),
			)

			manifestJSON, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal manifest: %w", err)
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "value: %s\n", manifestJSON); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
	}

	return cmd
}
