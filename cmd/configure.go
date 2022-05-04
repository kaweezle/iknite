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

import (
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	kustomizationDirectory = constants.DefaultKustomizationDirectory
)

// configureCmd represents the start command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure the cluster",
	Long: `Apply the configuration to the cluster using kustomize.

Checks if a /etc/iknite.d/kustomization.yaml file exists. In this case, it
applies the configuration in this directory. If there is no kustomization, it
applies the Embedded configuration that installs the following components:

- Flannel for networking.
- MetalLB for Load balancer services.
- Local-path provisionner to make PVCs avaiable.
- metrics-server to make resources work on payloads.

`,
	Run: performConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)

	// Here you will define your flags and configuration settings.

	configureCmd.Flags().StringVarP(&kustomizationDirectory, "directory", "d", constants.DefaultKustomizationDirectory,
		"The directory to look for kustomoization")
}

func performConfigure(cmd *cobra.Command, args []string) {

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	// We need to get it from root as we will apply configuration
	config, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	cobra.CheckErr(errors.Wrap(err, "While loading local cluster configuration"))
	cobra.CheckErr(config.CheckClusterRunning(1, 1, 0))

	context := log.Fields{
		"OutboundIP": ip,
	}
	cobra.CheckErr(provision.ApplyBaseKustomizations(kustomizationDirectory, context))

	log.Info("Base configuration applied")
}
