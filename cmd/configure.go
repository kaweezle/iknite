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
	"net"
	"time"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

var (
	kustomizationDirectory = constants.DefaultKustomizationDirectory
	waitTimeout            = 0
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

func initializeKustomization(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&kustomizationDirectory, "kustomize-directory", "d", constants.DefaultKustomizationDirectory,
		"The directory to look for kustomization. Can be an URL")
	viper.BindPFlag("kustomize_directory", cmd.Flags().Lookup("kustomize-directory"))
	cmd.Flags().IntVarP(&waitTimeout, "wait", "w", waitTimeout, "Wait n seconds for all pods to settle")
	cmd.Flags().BoolP("force-config", "C", false, "Force configuration even if it has already occured")
}

func init() {
	rootCmd.AddCommand(configureCmd)

	initializeKustomization(configureCmd)
}

func doConfiguration(ip net.IP, config *k8s.Config, force bool) error {
	client, err := config.Client()
	if err != nil {
		return err
	}
	cm, err := k8s.GetIkniteConfigMap(client)
	if err != nil {
		return err
	}
	if cm.Data["configured"] == "true" && !force {
		log.Info("configuration has already occured. Use -C to force.")
		return nil
	} else {
		context := log.Fields{
			"OutboundIP": ip,
		}
		var ids []resid.ResId
		var err error
		kustomizeDirectory := viper.GetString("kustomize_directory")
		if ids, err = provision.ApplyBaseKustomizations(kustomizeDirectory, context); err != nil {
			return err
		}

		cm.Data["configured"] = "true"
		_, err = k8s.WriteIkniteConfigMap(client, cm)
		if err != nil {
			return errors.Wrap(err, "While writing confiugration")
		}

		log.WithFields(log.Fields{
			"directory": kustomizeDirectory,
			"resources": ids,
		}).Info("Configuration applied")

		if waitTimeout > 0 {
			runtime.ErrorHandlers = runtime.ErrorHandlers[:0]
			return config.WaitForWorkloads(time.Second*time.Duration(waitTimeout), nil)
		}

	}

	return nil

}

func performConfigure(cmd *cobra.Command, args []string) {

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	// We need to get it from root as we will apply configuration
	config, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	cobra.CheckErr(errors.Wrap(err, "While loading local cluster configuration"))
	cobra.CheckErr(config.CheckClusterRunning(1, 1, 0))

	force, err := cmd.Flags().GetBool("force-config")
	cobra.CheckErr(err)
	cobra.CheckErr(doConfiguration(ip, config, force))
}
