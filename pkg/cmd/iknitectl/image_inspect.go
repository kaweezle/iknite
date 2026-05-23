package iknitectl

// cSpell: words imagesvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImageInspectCmd creates the image inspect command.
func CreateImageInspectCmd(deps *RootDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <image-ref>",
		Short: "Inspect image manifest details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := &imagesvc.Service{FS: deps.Host}
			result, err := service.Inspect(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("failed to inspect image: %w", err)
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "repository: %s\n", result.Repository); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "reference: %s\n", result.Reference); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "digest: %s\n", result.Descriptor.Digest.String()); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "layers: %d\n", len(result.Manifest.Layers)); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
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
