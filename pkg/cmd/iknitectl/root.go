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
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/utils"
)

// RootOptions contains configuration for the root command.
type RootOptions struct {
	host host.Host
	config.ConfigOptions
	util.BaseOptions
}

func NewRootOptions(e host.Host) *RootOptions {
	if e == nil {
		e = host.NewDefaultHost()
	}
	opts := &RootOptions{
		host:          e,
		BaseOptions:   *util.DefaultBaseOptions(),
		ConfigOptions: *config.NewConfigOptions(e),
	}
	return opts
}

// CreateRootCmd creates the root command with the given options.
func CreateRootCmd(opts *RootOptions) *cobra.Command {
	c, containerErr := NewContainer(opts)
	cobra.CheckErr(containerErr)

	rootCmd := &cobra.Command{
		Use:   "iknitectl",
		Short: "Development tools for iknite",
		Long: `iknitectl is a collection of development tools for the iknite project.

It provides utilities for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			rootCmd := cmd.Root()
			if err := ProvideCommand(c, rootCmd, args); err != nil {
				return fmt.Errorf("failed to provide command to container: %w", err)
			}
			if err := c.Invoke(func(cmdIf util.CmdInterface, opts *RootOptions) error {
				util.BindFlagsToViper(rootCmd, cmdIf)
				opts.SetUpLogs(cmd.OutOrStderr(), cmdIf)
				util.SetCmdInterface(cmd, cmdIf)
				err := util.InitializeConfiguration(rootCmd, cmdIf)
				if err != nil {
					return fmt.Errorf("failed to initialize configuration: %w", err)
				}
				// Re-setup logs after configuration is loaded to apply any log-related settings from the config file
				opts.SetUpLogs(cmd.OutOrStderr(), cmdIf)
				return nil
			}); err != nil {
				return fmt.Errorf("failed to set up configuration and logging: %w", err)
			}
			containerErr = c.Invoke(func(fe host.FileEnvironment, c *config.Config, opts *RootOptions) error {
				return opts.Resolve(fe, c)
			})
			if containerErr != nil {
				return fmt.Errorf("failed to resolve configuration: %w", containerErr)
			}
			return nil
		},

		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			return c.Invoke(func(hm *utils.HookManager) error {
				return hm.Run()
			})
		},
	}
	cobra.CheckErr(c.Invoke(func(opts *RootOptions) {
		rootCmd.SetOut(opts.Output)

		opts.AddFlags(rootCmd.PersistentFlags())
	}))

	// Add subcommands
	rootCmd.AddCommand(CreateEnvCmd(c.Scope("env")))
	rootCmd.AddCommand(CreateImageCmd(c.Scope("image")))
	rootCmd.AddCommand(CreateClusterCmd(c.Scope("cluster")))
	rootCmd.AddCommand(CreateWorkspaceCmd(c.Scope("workspace")))
	rootCmd.AddCommand(CreateAuthCmd(c.Scope("auth")))
	rootCmd.AddCommand(CreateBackendCmd(c.Scope("backend")))
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
