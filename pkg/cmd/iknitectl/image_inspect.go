package iknitectl

// cSpell: words imagesvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

type ImageRefRequest string

// CreateImageInspectCmd creates the image inspect command.
func CreateImageInspectCmd(s *dig.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <image-ref>",
		Short: "Inspect image manifest details",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			err := s.Provide(func() ImageRefRequest { return ImageRefRequest(args[0]) })
			if err != nil {
				return fmt.Errorf("failed to provide image reference: %w", err)
			}
			return s.Invoke(performImageInspect)
		},
	}

	return cmd
}

func performImageInspect(ctx context.Context, service *imagesvc.Service, imageRef string, out types.CmdOut) error {
	result, err := service.Inspect(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("failed to inspect image: %w", err)
	}
	logger := service.Logger
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
	if _, err = fmt.Fprintf(out, "value: %s\n", manifestJSON); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
