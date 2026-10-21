package iknitectl

// cSpell: words envsvc

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/dig"

	envsvc "github.com/kaweezle/iknite/pkg/iknitectl/env"
)

// EnvInitOptions contains flags for env init command.
type EnvInitOptions struct {
	Force      bool
	PrintPaths bool
}

// CreateEnvInitCmd creates the env init command.
func CreateEnvInitCmd(s *dig.Scope) *cobra.Command {
	opts := &EnvInitOptions{}
	cobra.CheckErr(s.Provide(func() *EnvInitOptions { return opts }))
	cobra.CheckErr(s.Provide(envsvc.NewService))

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize iknitectl working directory",
		RunE: func(_ *cobra.Command, _ []string) error {
			return s.Invoke(performEnvInit)
		},
	}

	addEnvInitCmdFlags(cmd.Flags(), opts)

	return cmd
}

func addEnvInitCmdFlags(flags *pflag.FlagSet, opts *EnvInitOptions) {
	flags.BoolVar(&opts.Force, "force", false, "Overwrite existing generated files")
	flags.BoolVar(&opts.PrintPaths, "print-paths", false, "Print resolved directory and file paths")
}

func performEnvInit(service *envsvc.Service, opts *EnvInitOptions) error {
	result, err := service.Init(&envsvc.InitRequest{
		Force:      opts.Force,
		PrintPaths: opts.PrintPaths,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	for _, message := range result.Messages {
		service.Logger.Info(message)
	}

	return nil
}
