package iknitectl

// cSpell: words envsvc

import (
	"fmt"

	"github.com/spf13/cobra"

	envsvc "github.com/kaweezle/iknite/pkg/iknitectl/env"
)

// EnvInitOptions contains flags for env init command.
type EnvInitOptions struct {
	ConfigDir      string
	Force          bool
	NonInteractive bool
	PrintPaths     bool
}

// CreateEnvInitCmd creates the env init command.
func CreateEnvInitCmd(deps *RootDependencies) *cobra.Command {
	opts := &EnvInitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize iknitectl working directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service := &envsvc.Service{
				FS:       deps.Host,
				Env:      deps.Env,
				Platform: deps.Platform,
			}

			result, err := service.Init(&envsvc.InitRequest{
				ConfigDir:      opts.ConfigDir,
				Force:          opts.Force,
				NonInteractive: opts.NonInteractive,
				PrintPaths:     opts.PrintPaths,
			})
			if err != nil {
				return fmt.Errorf("failed to initialize environment: %w", err)
			}

			for _, message := range result.Messages {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
					return fmt.Errorf("failed to print result: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.ConfigDir, "config-dir", "", "Override iknitectl working directory")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite existing generated files")
	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Disable prompts for CI usage")
	cmd.Flags().BoolVar(&opts.PrintPaths, "print-paths", false, "Print resolved directory and file paths")

	return cmd
}
