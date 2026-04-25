/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

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

// cSpell: disable
import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: enable

var (
	cfgFile       string
	IkniteVersion = "v0.5.2"
	Commit        = "unknown"
	BuildDate     = "unknown"
	BuiltBy       = "unknown"
)

// NewRootCmd creates a new root command.
func NewRootCmd(opts *util.BaseOptions) *cobra.Command {
	if opts == nil {
		opts = util.DefaultBaseOptions()
	}

	cobra.EnableTraverseRunHooks = true

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}

	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   "iknite",
		Short: "Start kubernetes in Alpine",
		Long: `Initializes Kubernetes in a WSL 2 Alpine distribution.
Makes the appropriate initialization of a WSL 2 Alpine distribution for running
kubernetes.`,
		Example: `> iknite start`,
		Version: IkniteVersion,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.SetUpLogs(os.Stderr); err != nil {
				return fmt.Errorf("while setting up logs: %w", err)
			}
			initConfig(cmd.Root())
			if err := config.DecodeIkniteConfig(ikniteConfig); err != nil {
				return fmt.Errorf("while decoding iknite config: %w", err)
			}

			return nil
		},
	}

	flags := rootCmd.PersistentFlags()
	opts.AddFlags(flags)
	util.AddConfigFlag(rootCmd)
	alpineHost := host.NewDefaultHost()

	rootCmd.AddCommand(NewKustomizeCmd(ikniteConfig, nil, nil, nil))
	rootCmd.AddCommand(newCmdInit(os.Stdout, nil, nil, alpineHost))
	rootCmd.AddCommand(newCmdReset(os.Stdin, os.Stdout, nil, nil))
	rootCmd.AddCommand(NewCmdClean(ikniteConfig, nil, alpineHost))
	rootCmd.AddCommand(NewKubeletCmd(ikniteConfig, nil, alpineHost))
	rootCmd.AddCommand(NewMdnsCmd(ikniteConfig))
	rootCmd.AddCommand(NewPrepareCommand(ikniteConfig))
	rootCmd.AddCommand(NewStartCmd(ikniteConfig, nil))
	rootCmd.AddCommand(NewStatusCmd(ikniteConfig, nil))
	rootCmd.AddCommand(NewInfoCmd(ikniteConfig))

	util.BindFlagsToViper(rootCmd, viper.GetViper())

	return rootCmd
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command) {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigType("yaml")
		viper.SetConfigName("iknite")
		viper.AddConfigPath("$HOME/.config/iknite/")
		viper.AddConfigPath("/etc/iknite.d/")
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix("iknite")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
	util.ApplyViperConfigToFlags(cmd, viper.GetViper())
}

func inheritsFlags(sourceFlags, targetFlags *pflag.FlagSet, cmdFlags ...string) {
	// If the list of flag to be inherited from the parent command is not defined, no flag is added
	if cmdFlags == nil {
		return
	}

	// add all the flags to be inherited to the target flagSet
	sourceFlags.VisitAll(func(f *pflag.Flag) {
		for _, c := range cmdFlags {
			if f.Name == c {
				targetFlags.AddFlag(f)
			}
		}
	})
}
