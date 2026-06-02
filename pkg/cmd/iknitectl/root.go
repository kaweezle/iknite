/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package iknitectl

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
)

// RootOptions contains configuration for the root command.
type RootOptions struct {
	host host.Host
	config.ConfigOptions
	util.BaseOptions
}

func NewRootOptions() *RootOptions {
	localHost := host.NewDefaultHost()
	opts := &RootOptions{
		BaseOptions:   *util.DefaultBaseOptions(),
		host:          localHost,
		ConfigOptions: *config.NewConfigOptions(localHost),
	}
	return opts
}

// CreateRootCmd creates the root command with the given options.
func CreateRootCmd(opts *RootOptions) *cobra.Command {
	if opts == nil {
		opts = NewRootOptions()
	}

	logger := opts.Logger()

	baseService := base.NewService(opts.host, logger, &opts.ConfigOptions)

	rootCmd := &cobra.Command{
		Use:   "iknitectl",
		Short: "Development tools for iknite",
		Long: `iknitectl is a collection of development tools for the iknite project.

It provides utilities for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmdIf := util.NewCmdInterface(&opts.BaseOptions)
			util.BindFlagsToViper(cmd.Root(), cmdIf)
			opts.SetUpLogs(cmd.OutOrStderr(), cmdIf)
			util.SetCmdInterface(cmd, cmdIf)
			err := util.InitializeConfiguration(cmd.Root(), cmdIf)
			if err != nil {
				return fmt.Errorf("failed to initialize configuration: %w", err)
			}
			// Re-setup logs after configuration is loaded to apply any log-related settings from the config file
			opts.SetUpLogs(cmd.OutOrStderr(), cmdIf)
			baseService.SetLogger(cmdIf.Logger())

			err = opts.Resolve(baseService.Host(), baseService.Config())
			if err != nil {
				return fmt.Errorf("failed to resolve configuration: %w", err)
			}
			return nil
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			if err := baseService.CloseStore(); err != nil {
				return fmt.Errorf("failed to close metadata store: %w", err)
			}
			return nil
		},
	}
	rootCmd.SetOut(opts.Output)

	opts.AddFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(CreateEnvCmd(baseService))
	rootCmd.AddCommand(CreateImageCmd(baseService))
	rootCmd.AddCommand(CreateClusterCmd(opts.host))
	rootCmd.AddCommand(CreateWorkspaceCmd(opts.host, opts.Output))
	rootCmd.AddCommand(CreateAuthCmd(opts.host))
	rootCmd.AddCommand(CreateBackendCmd(opts.host))
	util.AddConfigFlag(rootCmd)

	return rootCmd
}

func (opts *RootOptions) AddFlags(flags *pflag.FlagSet) {
	opts.BaseOptions.AddFlags(flags)
	opts.ConfigOptions.AddFlags(flags)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() { // nocov - This is the main entry point for the CLI, which is hard to test in CI
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cobra.CheckErr(CreateRootCmd(nil).ExecuteContext(ctx))
}
