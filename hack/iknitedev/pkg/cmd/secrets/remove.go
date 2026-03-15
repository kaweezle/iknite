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
	"github.com/spf13/cobra"

	pkgSecrets "github.com/kaweezle/iknite/hack/iknitedev/pkg/secrets"
)

func createSecretsRemoveCmd(opts *pkgSecrets.Options) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a secret key from the secrets file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return pkgSecrets.RemoveSecret(opts, args[0])
		},
	}
}
