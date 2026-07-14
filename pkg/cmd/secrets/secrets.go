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
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"go.uber.org/dig"

	"github.com/kaweezle/iknite/pkg/host"
	pkgSecrets "github.com/kaweezle/iknite/pkg/secrets"
)

// CreateSecretsCmd creates the secrets command.
func CreateSecretsCmd(s *dig.Scope, opts *pkgSecrets.Options) *cobra.Command {
	if opts == nil {
		opts = &pkgSecrets.Options{}
	}
	cobra.CheckErr(s.Provide(func() *pkgSecrets.Options {
		return opts
	}))

	secretsCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Read and modify values in a SOPS secrets file",
		Long: `Read and modify values in a SOPS encrypted secrets file.

Paths are specified in dot notation under the data key.
For example, github.api_token targets data.github.api_token.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return s.Decorate(func(
				opts *pkgSecrets.Options,
				logger *slog.Logger,
				fe host.FileEnvironment,
			) (*pkgSecrets.Options, error) {
				opts.Logger = logger
				opts.Fs = fe
				err := opts.SetDefaults()
				if err != nil {
					return nil, fmt.Errorf("error setting default opts: %w", err)
				}
				return opts, nil
			})
		},
	}

	secretsCmd.PersistentFlags().StringVarP(
		&opts.SecretsFile,
		"secrets-file",
		"s",
		pkgSecrets.DefaultSecretsFile,
		"Path to the SOPS secrets file",
	)

	secretsCmd.AddCommand(createSecretsGetCmd(s))
	secretsCmd.AddCommand(createSecretsSetCmd(s))
	secretsCmd.AddCommand(createSecretsRemoveCmd(s))
	secretsCmd.AddCommand(createSecretsInitCmd(s))

	return secretsCmd
}
