package iknitectl

// cSpell: words imagesvc

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	imagesvc "github.com/kaweezle/iknite/pkg/iknitectl/image"
	"github.com/kaweezle/iknite/pkg/utils"
)

// CreateImageListCmd creates the image ls command.
func CreateImageListCmd(baseService base.ServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List persisted images",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := db.Open(baseService.Config().Database)
			if err != nil {
				return fmt.Errorf("failed to open metadata store: %w", err)
			}

			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					//nolint:errcheck // We can only report close failures to stderr here.
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to close metadata store: %v\n", closeErr)
				}
			}()

			service := &imagesvc.Service{
				FS:     baseService.Host(),
				Logger: baseService.Logger(),
				Config: baseService.Config(),
				Store:  store,
			}
			items, err := service.ListImages()
			if err != nil {
				return fmt.Errorf("failed to list images: %w", err)
			}

			if len(items) == 0 {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), "No images found"); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				return nil
			}

			if _, err = fmt.Fprintln(cmd.OutOrStdout(), renderImageListTable(items)); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
	}

	return cmd
}

func renderImageListTable(items []imagesvc.ImageListItem) string {
	rows := make([]table.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, table.Row{
			item.Name,
			item.Source,
			item.Reference,
			// Keep for reference
			// item.Path,
			item.Artifacts,
			utils.FormatBytes(item.TotalSize),
			formatImageListTime(item.UpdatedAt),
		})
	}

	width := 180
	listTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "NAME", Width: 30},
			{Title: "SOURCE", Width: 36},
			{Title: "REF", Width: 20},
			// Keep for reference
			// {Title: "PATH", Width: 36},
			{Title: "ARTIFACTS", Width: 30},
			{Title: "SIZE", Width: 12},
			{Title: "UPDATED", Width: 24},
		}),
		table.WithRows(rows),
		table.WithWidth(width),
		table.WithHeight(len(items)+1),
	)

	return listTable.View()
}

func formatImageListTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format(time.RFC3339)
}
