package iknitectl

// cSpell: words imagesvc

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/cmd/types"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImagePullCmd creates the image pull command.
func CreateImagePullCmd(s *dig.Scope) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <image-ref>",
		Short: "Download image artifacts to the local filesystem",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := s.Provide(func() ImageRefRequest { return ImageRefRequest(args[0]) }); err != nil {
				return fmt.Errorf("providing image reference: %w", err)
			}
			return s.Invoke(performImagePull)
		},
	}

	return cmd
}

func performImagePull(
	ctx context.Context,
	service *imagesvc.Service,
	imageRef ImageRefRequest,
	out types.CmdOut,
) error {
	outputPath, err := service.Pull(ctx, &imagesvc.PullRequest{
		ImageRef: string(imageRef),
	})
	if err != nil {
		return fmt.Errorf("failed to pull image artifacts: %w", err)
	}

	if _, err = fmt.Fprintf(out, "saved image artifacts in %s\n", outputPath); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
