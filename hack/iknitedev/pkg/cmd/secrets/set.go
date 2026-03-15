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
	"io"
	"strings"

	"github.com/spf13/cobra"

	pkgSecrets "github.com/kaweezle/iknite/hack/iknitedev/pkg/secrets"
)

func createSecretsSetCmd(opts *pkgSecrets.Options) *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> [value]",
		Short: "Set a secret value in the secrets file",
		Long: `Set a secret value in the secrets file.

When value is omitted, it is read from stdin.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			value := ""
			if len(args) == 2 {
				value = args[1]
			} else {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("failed to read value from stdin: %w", err)
				}
				value = strings.TrimRight(string(data), "\r\n")
			}

			return pkgSecrets.SetSecret(opts, args[0], value)
		},
	}
}
