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
// cSpell: words sirupsen logrus wrapcheck
package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/cmd/options"
)

const (
	ConfigSectionAnnotation = "config-section"
	SkipViperBindAnnotation = "skip-viper-bind"
)

// GetBaseDirectory returns the first parent directory that contains a .git directory.
func GetBaseDirectory() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("while getting base directory: %w", err)
	}
	for dir != "/" {
		if _, err := os.Stat(fmt.Sprintf("%s/.git", dir)); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return ".", nil
}

// SetCommandConfigSection sets the annotation for the command to indicate that it has a configuration section.
func SetCommandConfigSection(cmd *cobra.Command, section string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}

	cmd.Annotations[ConfigSectionAnnotation] = section
}

// CommandHasConfigSection checks if the command has a configuration section.
func CommandHasConfigSection(cmd *cobra.Command) bool {
	if cmd.Annotations == nil {
		return false
	}
	_, ok := cmd.Annotations[ConfigSectionAnnotation]
	return ok
}

// CommandConfigSection returns the configuration section for the command.
func CommandConfigSection(cmd *cobra.Command) string {
	if cmd.Annotations == nil {
		return ""
	}
	val, ok := cmd.Annotations[ConfigSectionAnnotation]
	if !ok {
		return ""
	}
	return val
}

// Add the viper binding skip annotation to the command.
func SetSkipViperBindForCommand(cmd *cobra.Command, skip bool) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}

	cmd.Annotations[SkipViperBindAnnotation] = fmt.Sprintf("%t", skip)
}

// Add the viper binding skip annotation to the flag.
func SetSkipViperBindForFlag(flag *pflag.Flag, skip bool) {
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}

	flag.Annotations[SkipViperBindAnnotation] = []string{fmt.Sprintf("%t", skip)}
}

// CmdShouldSkipViperBind checks if the command has a skip viper bind annotation.
func CmdShouldSkipViperBind(cmd *cobra.Command) bool {
	if cmd.Annotations == nil {
		return false
	}
	val, ok := cmd.Annotations[SkipViperBindAnnotation]
	if !ok {
		return false
	}
	return val == "true"
}

// SkipViperBind checks if the flag has a skip viper bind annotation.
func FlagShouldSkipViperBind(flag *pflag.Flag) bool {
	if flag.Annotations == nil {
		return false
	}
	val, ok := flag.Annotations[SkipViperBindAnnotation]
	if !ok {
		return false
	}
	return val[0] == "true"
}

func toStringSlice(val any) ([]string, error) {
	switch v := val.(type) {
	case []string:
		return v, nil
	// For the case where the value is defined in a environment variable.
	// see https://github.com/spf13/viper/issues/380
	case string:
		return strings.Split(v, ","), nil
	case []any:
		values := make([]string, len(v))
		for i, item := range v {
			values[i] = fmt.Sprintf("%v", item)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected slice value, got %T", val)
	}
}

// BindFlagValue applies the viper config value to the flag when the flag is not set and viper has a value.
func BindFlagValue(f *pflag.Flag, v *viper.Viper, viperName string) error {
	// Apply the viper config value to the flag when the flag is not set and viper has a value
	if f.Changed || !v.IsSet(viperName) {
		logrus.WithFields(logrus.Fields{
			"option":    f.Name,
			"viper_key": viperName,
		}).Debug("skipping applying viper config to flag because flag is already set or viper key is not set")
		return nil
	}
	val := v.Get(viperName)

	if vi, ok := f.Value.(pflag.SliceValue); ok {
		stringValues, err := toStringSlice(val)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"option":    f.Name,
				"viper_key": viperName,
				"value":     val,
			}).WithError(err).Error("error converting options")
			return fmt.Errorf("while getting viper array value for %s: %w", viperName, err)
		}
		if err := vi.Replace(stringValues); err != nil {
			logrus.WithFields(logrus.Fields{
				"option":    f.Name,
				"viper_key": viperName,
				"value":     val,
			}).WithError(err).Error("error replacing options")
			return fmt.Errorf(
				"while replacing viper array value for %s from viper key %s with value %v: %w",
				f.Name,
				viperName,
				val,
				err,
			)
		}
	} else {
		if err := f.Value.Set(fmt.Sprintf("%v", val)); err != nil {
			logrus.WithFields(logrus.Fields{
				"option":    f.Name,
				"viper_key": viperName,
				"value":     val,
			}).WithError(err).Error("error replacing options")
			return fmt.Errorf(
				"while setting viper value for %s from viper key %s with value %v: %w",
				f.Name,
				viperName,
				val,
				err,
			)
		}
	}
	return nil
}

func BindFlag(f *pflag.Flag, v *viper.Viper, viperName string) error {
	return v.BindPFlag(viperName, f) //nolint:wrapcheck // no added value from wrapping error
}

// BindFlags binds each cobra flag to its associated viper configuration (config file and environment variable).
func BindFlags(
	cmd *cobra.Command,
	v *viper.Viper,
	prefix string,
	binder func(f *pflag.Flag, v *viper.Viper, viperName string) error,
) {
	if CmdShouldSkipViperBind(cmd) {
		return
	}
	if CommandHasConfigSection(cmd) {
		prefix = CommandConfigSection(cmd) + "."
	}

	persistent := cmd.PersistentFlags()
	persistent.VisitAll(func(f *pflag.Flag) {
		if !FlagShouldSkipViperBind(f) {
			// Environment variables can't have dashes in them, so bind them to their equivalent
			// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
			viperName := prefix + strings.ReplaceAll(f.Name, "-", "_")
			if err := binder(f, v, viperName); err != nil {
				logrus.WithFields(logrus.Fields{
					"option":    f.Name,
					"viper_key": viperName,
				}).WithError(err).Error("error binding flag to viper")
			}
		}
	})

	flags := cmd.Flags()
	// Same with the command flags
	flags.VisitAll(func(f *pflag.Flag) {
		if !FlagShouldSkipViperBind(f) {
			viperName := prefix + strings.ReplaceAll(f.Name, "-", "_")
			if err := binder(f, v, viperName); err != nil {
				logrus.WithFields(logrus.Fields{
					"option":    f.Name,
					"viper_key": viperName,
				}).WithError(err).Error("error binding flag to viper")
			}
		}
	})
	// visit the subcommands
	for _, c := range cmd.Commands() {
		BindFlags(c, v, prefix, binder)
	}
}

// BindFlagsToViper binds each cobra flag to its associated viper configuration (config file and environment variable).
func BindFlagsToViper(cmd *cobra.Command, v *viper.Viper) {
	BindFlags(cmd, v, "", BindFlag)
}

// ApplyViperConfigToFlags applies the viper configuration to the flags of the command when the flags are not set.
// This allows the configuration file and environment variables to override the default flag values,
// but still allow the user to override them with command line flags.
func ApplyViperConfigToFlags(cmd *cobra.Command, v *viper.Viper) {
	BindFlags(cmd, v, "", BindFlagValue)
}

// GetConfigDirectory returns the directory where the configuration file should be stored based on the
// operating system conventions.
func GetConfigDirectory(commandName string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}
		return filepath.Join(appData, commandName), nil
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("while getting user home directory: %w", err)
		}
		return filepath.Join(homeDir, "Library", "Application Support", commandName), nil
	default: // Linux and other Unix-like systems
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("while getting user home directory: %w", err)
			}
			xdgConfigHome = filepath.Join(homeDir, ".config")
		}
		return filepath.Join(xdgConfigHome, commandName), nil
	}
}

// InitializeConfiguration reads in config file and ENV variables if set.
func InitializeConfiguration(rootCmd *cobra.Command) error {
	commandName := rootCmd.Name()
	envPrefix := strings.ToUpper(commandName)
	configFileFlag := rootCmd.PersistentFlags().Lookup(options.Config)

	if configFileFlag != nil && configFileFlag.Value.String() != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFileFlag.Value.String())
	} else {
		// Search config in home directory with name ".<commandName>" (without extension).
		viper.SetConfigName("." + commandName)
		configDirectory, err := GetConfigDirectory(commandName)
		if err != nil {
			return fmt.Errorf("while getting configuration directory: %w", err)
		}
		viper.AddConfigPath(configDirectory)
		if baseDir, err := GetBaseDirectory(); err == nil {
			viper.AddConfigPath(baseDir) // adding current directory as first search path
		} else {
			logrus.WithError(err).Warn("could not determine base directory for config file search, skipping")
		}
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		logrus.WithError(err).Debug("could not read config file, skipping")
	}
	return nil
}
