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
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	kustomization                = constants.DefaultKustomization
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
	PreRun: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		viper.BindPFlag(config.Kustomization, flags.Lookup(options.Kustomization))
		viper.BindPFlag(config.ForceConfig, flags.Lookup(options.ForceConfig))
	},
}

func initializeKustomization(flagSet *flag.FlagSet) {
	flagSet.IntVarP(&waitTimeout, options.Wait, "w", waitTimeout, "Wait n seconds for all pods to settle")
	flagSet.BoolP(options.ForceConfig, "C", false, "Force configuration even if it has already occurred")
	flagSet.IntVar(&clusterCheckWaitMilliseconds, options.ClusterCheckWait, clusterCheckWaitMilliseconds, "Milliseconds to wait between each cluster check")
	flagSet.IntVar(&clusterCheckRetries, options.ClusterCheckRetries, clusterCheckRetries, "Number of tries to access the cluster")
	flagSet.IntVar(&clusterCheckOkResponses, options.ClusterCheckOkResponses, clusterCheckOkResponses, "Number of Ok response to receive before proceeding")
}

func init() {
	rootCmd.AddCommand(configureCmd)

	initializeKustomization(configureCmd.Flags())
	configureCmd.Flags().StringVarP(&kustomization, options.Kustomization, "d", constants.DefaultKustomization,
		"The directory to look for kustomization. Can be an URL")
}

func performConfigure(cmd *cobra.Command, args []string) {

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	// We need to get it from root as we will apply configuration
	kubeConfig, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	cobra.CheckErr(errors.Wrap(err, "While loading local cluster configuration"))
	cobra.CheckErr(kubeConfig.CheckClusterRunning(clusterCheckRetries, clusterCheckOkResponses, clusterCheckWaitMilliseconds))

	force := viper.GetBool(config.ForceConfig)
	kustomization := viper.GetString(config.Kustomization)
	cobra.CheckErr(kubeConfig.DoConfiguration(ip, kustomization, force, waitTimeout))
}
