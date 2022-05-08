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
	"os"
	"time"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/crio"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Creates or starts the cluster",
	Long: `Starts the cluster. Performs the following operations:

- Starts OpenRC,
- Starts CRI-O,
- If Kubelet has never been started, execute kubeadm init to provision
  the cluster,
- Allows the use of kubectl from the root account,
- Installs flannel, metal-lb and local-path-provisioner.
`,
	Run: perform,
}

var waitTimeout = 0
var minimumPodsReady = 6

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	startCmd.Flags().IntVarP(&waitTimeout, "wait", "w", waitTimeout, "Wait n seconds for all pods to settle")
	startCmd.Flags().IntVarP(&minimumPodsReady, "minimum-pods", "m", minimumPodsReady, "Minimal number of pods")

}

func perform(cmd *cobra.Command, args []string) {

	// Allow forwarding (kubeadm requirement)
	utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	cobra.CheckErr(alpine.EnsureNetFilter())

	// Start OpenRC
	cobra.CheckErr(alpine.StartOpenRC())

	cobra.CheckErr(alpine.EnableService(constants.CrioServiceName))
	cobra.CheckErr(alpine.EnableService(constants.KubeletServiceName))
	cobra.CheckErr(alpine.StartService(constants.CrioServiceName))

	// CRI-O is started by OpenRC
	available, err := crio.WaitForCrio()
	cobra.CheckErr(err)
	if !available {
		log.Fatal("CRI-O not available")
	}

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	exist := false
	config, err := k8s.LoadFromDefault()
	if err == nil {
		if config.IsConfigServerAddress(ip) {
			exist = true
		} else {
			cobra.CheckErr(alpine.StopService(constants.KubeletServiceName))
			cobra.CheckErr(k8s.CleanConfig())
		}
	} else {
		if !os.IsNotExist(err) {
			cobra.CheckErr(errors.Wrap(err, "While loading existing kubeconfig"))
		}
	}

	if !exist {
		restartProxy := err == nil
		cobra.CheckErr(alpine.FixKubeletConfiguration())
		cobra.CheckErr(k8s.RunKubeadmInit(ip))
		config, err = k8s.LoadFromDefault()
		cobra.CheckErr(err)
		if restartProxy {
			log.Info("Restart kube-proxy")
			cobra.CheckErr(config.RestartProxy())
		}
	} else {
		// The service should have been started by OpenRC
		log.Info("Waiting for service to start...")
		cobra.CheckErr(config.CheckClusterRunning(10, 2, 2))
	}

	cobra.CheckErr(config.RenameConfig(ClusterName).WriteToFile(constants.KubernetesRootConfig))

	// Apply base customization. This adds the following to the cluster
	// - MetalLB
	// - Flannel
	// - Local path provisioner
	// - Metrics server
	// The outbound ip address is needed for MetalLB.
	context := log.Fields{
		"OutboundIP": ip,
	}
	var ids []resid.ResId
	ids, err = provision.ApplyBaseKustomizations(constants.DefaultKustomizationDirectory, context)
	cobra.CheckErr(err)
	log.WithFields(log.Fields{
		"directory": constants.DefaultKustomizationDirectory,
		"resources": ids,
	}).Info("Configuration applied")

	if waitTimeout > 0 {
		cobra.CheckErr(config.WaitForCluster(time.Second*time.Duration(waitTimeout), minimumPodsReady))
	}

	log.Info("executed")
}
