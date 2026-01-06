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
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
)

// cSpell: enable

var (
	cfgFile  string
	v        string
	jsonLogs bool
)

// NewRootCmd creates a new root command.
func NewRootCmd() *cobra.Command {
	cobra.OnInitialize(initConfig)
	cobra.EnableTraverseRunHooks = true

	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   "iknite",
		Short: "Start kubernetes in Alpine",
		Long: `Initializes Kubernetes in a WSL 2 Alpine distribution.
Makes the appropriate initialization of a WSL 2 Alpine distribution for running
kubernetes.`,
		Example: `> iknite start`,
		Version: "v0.5.2", // <---VERSION--->
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
	}

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := SetUpLogs(os.Stderr, v, jsonLogs); err != nil {
			return err
		}
		return nil
	}
	flags := rootCmd.PersistentFlags()

	flags.StringVar(&cfgFile, options.Config, "", "config file (default is $HOME/.config/iknite/iknite.yaml or /etc/iknite.d/iknite.yaml)")
	flags.StringVarP(&v, options.Verbosity, "v", log.InfoLevel.String(), "Log level (debug, info, warn, error, fatal, panic)")
	flags.BoolVar(&jsonLogs, options.Json, false, "Log messages in JSON")

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}

	rootCmd.AddCommand(NewKustomizeCmd())
	rootCmd.AddCommand(newCmdInit(os.Stdout, nil))
	rootCmd.AddCommand(newCmdReset(os.Stdin, os.Stdout, nil))
	rootCmd.AddCommand(NewCmdClean(ikniteConfig, nil))
	rootCmd.AddCommand(NewKubeletCmd(ikniteConfig))
	rootCmd.AddCommand(NewMdnsCmd())
	rootCmd.AddCommand(NewPrepareCommand(ikniteConfig))
	rootCmd.AddCommand(NewStartCmd(ikniteConfig))
	rootCmd.AddCommand(NewStatusCmd(ikniteConfig))

	return rootCmd
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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
}

func SetUpLogs(out io.Writer, level string, json bool) error {
	log.SetOutput(out)
	if json {
		log.SetFormatter(&log.JSONFormatter{})
	}
	lvl, err := log.ParseLevel(v)
	if err != nil {
		return errors.Wrap(err, "parsing log level")
	}
	log.SetLevel(lvl)
	return nil
}
