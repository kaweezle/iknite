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
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/cmd/secrets"
	"github.com/kaweezle/iknite/pkg/cmd/util"
)

// RootOptions contains configuration for the root command.
type RootOptions struct {
	Fs  afero.Fs
	out io.Writer
	util.BaseOptions
}

func NewRootOptions() *RootOptions {
	opts := &RootOptions{
		BaseOptions: *util.DefaultBaseOptions(),
		Fs:          afero.NewOsFs(),
		out:         os.Stdout,
	}
	return opts
}

// CreateRootCmd creates the root command with the given options.
func CreateRootCmd(opts *RootOptions) *cobra.Command {
	if opts == nil {
		opts = NewRootOptions()
	}

	rootCmd := &cobra.Command{
		Use:   "iknitectl",
		Short: "Development tools for iknite",
		Long: `iknitectl is a collection of development tools for the iknite project.

It provides utilities for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			err := opts.SetUpLogs(opts.out)
			if err != nil {
				return fmt.Errorf("failed to set up logs: %w", err)
			}
			err = util.InitializeConfiguration(cmd.Root(), viper.GetViper())
			if err != nil {
				return fmt.Errorf("failed to initialize configuration: %w", err)
			}
			// Re-setup logs after configuration is loaded to apply any log-related settings from the config file
			err = opts.SetUpLogs(opts.out)
			if err != nil {
				return fmt.Errorf("failed to set up logs: %w", err)
			}
			return nil
		},
	}

	opts.AddFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(CreateInstallCmd(opts.Fs))
	rootCmd.AddCommand(CreateKustomizeCmd(opts.Fs, opts.out))
	rootCmd.AddCommand(CreateApplicationCmd(opts.Fs, opts.out))
	rootCmd.AddCommand(secrets.CreateSecretsCmd(opts.Fs, nil))
	util.AddConfigFlag(rootCmd)

	util.BindFlagsToViper(rootCmd, viper.GetViper())

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(CreateRootCmd(nil).Execute())
}
