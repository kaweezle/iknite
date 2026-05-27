package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
)

const (
	DefaultCACommonName   = "iknite-ca"
	DefaultCAOrganization = "iknite"
	DefaultCAName         = "ca"
)

type ConfigOptions struct {
	ConfigDir      string
	CAName         string
	CACommonName   string
	CAOrganization []string
}

func NewConfigOptions(e host.FileEnvironment) *ConfigOptions {
	configDir, err := DefaultConfigDir(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to determine default config directory: %v\n", err)
		configDir = "" // Fallback to empty string if we can't determine the config directory
	}
	return &ConfigOptions{
		ConfigDir:      configDir,
		CAName:         DefaultCAName,
		CACommonName:   DefaultCACommonName,
		CAOrganization: []string{DefaultCAOrganization},
	}
}

func DefaultConfigDir(fse host.Environment) (string, error) {
	configDir, err := fse.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return fse.JoinPath(configDir, constants.IkniteConfName), nil
}

func (o *ConfigOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ConfigDir, "config-dir", "", "Path to iknite config directory (defaults to $HOME/.iknite)")
	flags.StringVar(&o.CAName, "ca-name", DefaultCAName, "Name of the Certificate Authority (CA)")
	flags.StringVar(&o.CACommonName, "ca-common-name", DefaultCACommonName,
		"Common name of the Certificate Authority (CA)")
	flags.StringSliceVar(&o.CAOrganization, "ca-organization", []string{DefaultCAOrganization},
		"Organization of the Certificate Authority (CA)")
}

func (o *ConfigOptions) Resolve(fse host.FileEnvironment, paths *Config) error {
	if o.ConfigDir == "" {
		var err error
		o.ConfigDir, err = DefaultConfigDir(fse)
		if err != nil {
			return fmt.Errorf("failed to get default config directory: %w", err)
		}
	}
	configDir := filepath.Clean(o.ConfigDir)
	paths.Root = configDir

	paths.Auth = fse.JoinPath(configDir, DefaultAuthDirname)
	paths.Shared = fse.JoinPath(configDir, DefaultSharedDirname)
	paths.Images = fse.JoinPath(configDir, DefaultImagesDirname)
	paths.Clusters = fse.JoinPath(configDir, DefaultClustersDirname)

	paths.CA.CommonName = o.CACommonName
	paths.CA.Organization = o.CAOrganization
	paths.CA.CertPath = pki.PathForCert(paths.Auth, o.CAName)
	paths.CA.KeyPath = pki.PathForKey(paths.Auth, o.CAName)

	paths.SharedSecrets = fse.JoinPath(paths.Shared, DefaultSecretsFilename)
	paths.SharedSecretsKey = fse.JoinPath(paths.Shared, DefaultKeyFilename)
	paths.SharedValues = fse.JoinPath(paths.Shared, DefaultValuesFilename)

	return nil
}
