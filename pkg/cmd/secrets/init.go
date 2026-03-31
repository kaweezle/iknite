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

	"github.com/spf13/cobra"

	pkgSecrets "github.com/kaweezle/iknite/pkg/secrets"
)

func createSecretsInitCmd(opts *pkgSecrets.Options) *cobra.Command {
	defaultKeyFile := opts.KeyFile
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .sops.yaml, secrets.sops.yaml, and an SSH key pair",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := pkgSecrets.InitSecrets(opts)
			if err != nil {
				return fmt.Errorf("failed to initialize secrets: %w", err)
			}

			for _, message := range result.Messages {
				if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), message); writeErr != nil {
					return fmt.Errorf("error while outputting result: %w", writeErr)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Overwrite existing generated files")
	cmd.Flags().StringVarP(&opts.KeyFile, "key-file", "k", defaultKeyFile, "SSH private key file to use or generate")

	return cmd
}
