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
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootOptions contains configuration for the root command.
type RootOptions struct {
	Fs      afero.Fs
	CfgFile string
}

// CreateRootCmd creates the root command with the given options.
func CreateRootCmd(opts *RootOptions) *cobra.Command {
	if opts == nil {
		opts = &RootOptions{
			Fs: afero.NewOsFs(),
		}
	}

	rootCmd := &cobra.Command{
		Use:   "iknitedev",
		Short: "Development tools for iknite",
		Long: `iknitedev is a collection of development tools for the iknite project.

It provides utilities for managing secrets, building artifacts, and other
development tasks that are not part of the main iknite binary.`,
	}

	cobra.OnInitialize(func() { initConfig(opts.CfgFile) })

	rootCmd.PersistentFlags().
		StringVar(&opts.CfgFile, "config", "", "config file (default is $HOME/.iknitedev.yaml)")

	// Add subcommands
	rootCmd.AddCommand(CreateInstallCmd(opts.Fs))

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(CreateRootCmd(nil).Execute())
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cfgFile string) {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".iknitedev" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".iknitedev")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
