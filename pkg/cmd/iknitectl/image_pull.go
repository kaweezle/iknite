package iknitectl

// cSpell: words imagesvc

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
)

// CreateImagePullCmd creates the image pull command.
func CreateImagePullCmd(baseService base.ServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <image-ref>",
		Short: "Download image artifacts to the local filesystem",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := db.Open(baseService.Config().Database)
			if err != nil {
				return fmt.Errorf("failed to open metadata store: %w", err)
			}

			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					//nolint:errcheck // We can't do much about a close error at this point, so we'll just log it.
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to close metadata store: %v\n", closeErr)
				}
			}()

			service := &imagesvc.Service{
				FS:     baseService.Host(),
				Logger: baseService.Logger(),
				Config: baseService.Config(),
				Store:  store,
			}
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
