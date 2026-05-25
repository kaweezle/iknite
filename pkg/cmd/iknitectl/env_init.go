package iknitectl

// cSpell: words envsvc

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/host"
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
func CreateEnvInitCmd(localHost host.Host) *cobra.Command {
	opts := &EnvInitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize iknitectl working directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service := &envsvc.Service{
				FS:     localHost,
				Logger: util.LoggerFromContext(cmd.Context()),
			}
			return performEnvInit(service, opts)
		},
	}

	addEnvInitCmdFlags(cmd.Flags(), opts)

	return cmd
}

func addEnvInitCmdFlags(flags *pflag.FlagSet, opts *EnvInitOptions) {
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "Override iknitectl working directory")
	flags.BoolVar(&opts.Force, "force", false, "Overwrite existing generated files")
	flags.BoolVar(&opts.NonInteractive, "non-interactive", false, "Disable prompts for CI usage")
	flags.BoolVar(&opts.PrintPaths, "print-paths", false, "Print resolved directory and file paths")
}

func performEnvInit(service *envsvc.Service, opts *EnvInitOptions) error {
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
		service.Logger.Info(message)
	}

	return nil
}
