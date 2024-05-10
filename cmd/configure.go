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
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	kustomizationDirectory       = constants.DefaultKustomizationDirectory
	waitTimeout                  = 0
	clusterCheckWaitMilliseconds = 500
	clusterCheckRetries          = 1
	clusterCheckOkResponses      = 1
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
- Local-path provisioner to make PVCs available.
- metrics-server to make resources work on payloads.

`,
	Run: performConfigure,
}

func initializeKustomization(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&kustomizationDirectory, "kustomize-directory", "d", constants.DefaultKustomizationDirectory,
		"The directory to look for kustomization. Can be an URL")
	viper.BindPFlag("kustomize_directory", cmd.Flags().Lookup("kustomize-directory"))
	cmd.Flags().IntVarP(&waitTimeout, "wait", "w", waitTimeout, "Wait n seconds for all pods to settle")
	cmd.Flags().BoolP("force-config", "C", false, "Force configuration even if it has already occurred")
	viper.BindPFlag("force_config", cmd.Flags().Lookup("force-config"))
	cmd.Flags().IntVar(&clusterCheckWaitMilliseconds, "cluster-check-wait", clusterCheckWaitMilliseconds, "Milliseconds to wait between each cluster check")
	cmd.Flags().IntVar(&clusterCheckRetries, "cluster-check-retries", clusterCheckRetries, "Number of tries to access the cluster")
	cmd.Flags().IntVar(&clusterCheckOkResponses, "cluster-check-ok-responses", clusterCheckOkResponses, "Number of Ok response to receive before proceeding")
}

func init() {
	rootCmd.AddCommand(configureCmd)

	initializeKustomization(configureCmd)
}

func performConfigure(cmd *cobra.Command, args []string) {

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	// We need to get it from root as we will apply configuration
	config, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	cobra.CheckErr(errors.Wrap(err, "While loading local cluster configuration"))
	cobra.CheckErr(config.CheckClusterRunning(clusterCheckRetries, clusterCheckOkResponses, clusterCheckWaitMilliseconds))

	force := viper.GetBool("force_config")
	cobra.CheckErr(config.DoConfiguration(ip, force, waitTimeout))
}
