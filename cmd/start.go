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

	"github.com/antoinemartin/k8wsl/pkg/alpine"
	"github.com/antoinemartin/k8wsl/pkg/constants"
	"github.com/antoinemartin/k8wsl/pkg/crio"
	"github.com/antoinemartin/k8wsl/pkg/k8s"
	"github.com/antoinemartin/k8wsl/pkg/provision"
	"github.com/antoinemartin/k8wsl/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
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

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func perform(cmd *cobra.Command, args []string) {

	// Start OpenRC
	cobra.CheckErr(alpine.StartOpenRC())

	// Networking is already started so we pretend the service has been run.
	cobra.CheckErr(alpine.PretendServiceStarted("networking"))

	// Enable CRIO and Kubelet. Kubelet will be started by kubeadm or by us
	cobra.CheckErr(alpine.EnableService(constants.CrioServiceName))
	cobra.CheckErr(alpine.EnableService(constants.KubeletServiceName))

	// We don't mess with IPV6
	cobra.CheckErr(utils.MoveFileIfExists("/etc/cni/net.d/10-crio-bridge.conf", "/etc/cni/net.d/12-crio-bridge.conf"))

	// Allow forwarding (kubeadm requirement)
	utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	// We need to start CRI-O
	cobra.CheckErr(alpine.StartService("crio"))
	available, err := crio.WaitForCrio()
	cobra.CheckErr(err)
	if !available {
		log.Fatal("CRI-O not available")
	}

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	exist := false
	config, err := clientcmd.LoadFromFile(constants.KubernetesAdminConfig)
	if err == nil {
		if k8s.IsConfigServerAddress(config, ip) {
			exist = true
		} else {
			cobra.CheckErr(k8s.CleanConfig())
		}
	} else {
		if !os.IsNotExist(err) {
			cobra.CheckErr(errors.Wrap(err, "While loading existing kubeconfig"))
		}
	}

	if !exist {
		cobra.CheckErr(k8s.RunKubeadmInit(ip))
		config, err = clientcmd.LoadFromFile(constants.KubernetesAdminConfig)
		cobra.CheckErr(err)
	} else {
		// Just start the service
		log.Info("Starting the kubelet service...")
		cobra.CheckErr(alpine.StartService(constants.KubeletServiceName))
		// TODO: Need to wait for node to be ready
		log.Info("Waiting for service to start...")
		cobra.CheckErr(k8s.CheckClusterRunning(config))
	}

	// TODO: Check that cluster is Ok
	cobra.CheckErr(clientcmd.WriteToFile(*k8s.RenameConfig(config, "k8wsl"), "/root/.kube/config"))

	// Untaint master. It needs a valid kubeconfig
	/*if out, err := exec.Command(c.KubectlCmd, "taint", "nodes", "--all", "node-role.kubernetes.io/master-").CombinedOutput(); err != nil {
		if strout := string(out); strout != "error: taint \"node-role.kubernetes.io/master\" not found\n" {
			log.Fatal(err, strout)
		}
	}*/

	// Apply base customization. This adds the following to the cluster
	// - MetalLB
	// - Flannel
	// - Local path provisioner
	// - Metrics server
	// The outbound ip address is needed for MetalLB.
	context := log.Fields{
		"OutboundIP": ip,
	}
	cobra.CheckErr(provision.ApplyBaseKustomizations(context))

	log.Info("executed")
}
