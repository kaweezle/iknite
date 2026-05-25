package iknitectl

// cSpell: words imagesvc

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/host"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImagePullCmd creates the image pull command.
func CreateImagePullCmd(localHost host.Host) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <image-ref>",
		Short: "Download image artifacts to the local filesystem",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := &imagesvc.Service{FS: localHost}
			outputPath, err := service.Pull(cmd.Context(), &imagesvc.PullRequest{
				ImageRef: args[0],
			})
			if err != nil {
				return fmt.Errorf("failed to pull image artifacts: %w", err)
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "saved image artifacts in %s\n", outputPath); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
	}

	return cmd
}
