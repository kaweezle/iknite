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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

func NewPrepareCommand(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	// prepareCmd represents the start command
	prepareCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the VM for Kubernetes",
		Long: `Prepare the VM for Kubernetes. Performs the following operations:

- Ensures the appropriate kernel modules are loaded,
- Ensures the VM has a machine ID,
- Ensures the Virtual IP address is set and mapped to the hostname,
- Ensures Iknite is started with OpenRC.
`,
		PersistentPreRun: config.StartPersistentPreRun,
		Run:              func(_ *cobra.Command, _ []string) { performPrepare(ikniteConfig) },
	}
	flags := prepareCmd.Flags()

	config.ConfigureClusterCommand(flags, ikniteConfig)

	return prepareCmd
}

func performPrepare(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	cobra.CheckErr(config.DecodeIkniteConfig(ikniteConfig))
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ikniteConfig))
	log.Info("VM is ready")
}
