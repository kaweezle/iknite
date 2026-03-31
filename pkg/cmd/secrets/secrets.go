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
package secrets

import (
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	pkgSecrets "github.com/kaweezle/iknite/pkg/secrets"
)

// CreateSecretsCmd creates the secrets command.
func CreateSecretsCmd(fs afero.Fs, opts *pkgSecrets.Options) *cobra.Command {
	if opts == nil {
		opts = &pkgSecrets.Options{}
	}
	if opts.Fs == nil {
		opts.Fs = fs
	}
	if opts.HomeDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			opts.HomeDir = homeDir
		}
	}
	if opts.SecretsFile == "" {
		opts.SecretsFile = pkgSecrets.DefaultSecretsFile
	}

	defaultSecretsFile := opts.SecretsFile

	secretsCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Read and modify values in a SOPS secrets file",
		Long: `Read and modify values in a SOPS encrypted secrets file.

Paths are specified in dot notation under the data key.
For example, github.api_token targets data.github.api_token.`,
	}

	secretsCmd.PersistentFlags().StringVarP(
		&opts.SecretsFile,
		"secrets-file",
		"s",
		defaultSecretsFile,
		"Path to the SOPS secrets file",
	)

	secretsCmd.AddCommand(createSecretsGetCmd(opts))
	secretsCmd.AddCommand(createSecretsSetCmd(opts))
	secretsCmd.AddCommand(createSecretsRemoveCmd(opts))
	secretsCmd.AddCommand(createSecretsInitCmd(opts))

	return secretsCmd
}
