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
	"fmt"
	"net"
	"os"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/crio"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/txn2/txeh"
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
	PersistentPreRun: startPersistentPreRun,
	Run:              perform,
}

func init() {
	rootCmd.AddCommand(startCmd)

	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))
	domain_name := ""

	wsl := utils.IsOnWSL()
	if wsl {
		ip = net.ParseIP(constants.WSLIPAddress)
		domain_name = constants.WSLHostName
	}

	startCmd.Flags().String("ip", ip.String(), "Cluster IP address")
	startCmd.Flags().Bool("ip-create", wsl, "Add IP address if it doesn't exist")
	startCmd.Flags().String("ip-network-interface", "eth0", "Interface to which add IP")
	startCmd.Flags().String("domain-name", domain_name, "Domain name of the cluster")
	startCmd.Flags().Bool("enable-mdns", wsl, "Enable mDNS publication of domain name")
	startCmd.Flags().StringVar(&k8s.KubernetesVersion, "kubernetes-version", k8s.KubernetesVersion, "Kubernetes version to install")

	initializeKustomization(startCmd)
}

func startPersistentPreRun(cmd *cobra.Command, args []string) {

	viper.BindPFlag("cluster.ip", cmd.Flags().Lookup("ip"))
	viper.BindPFlag("cluster.create_ip", cmd.Flags().Lookup("ip-create"))
	viper.BindPFlag("cluster.network_interface", cmd.Flags().Lookup("ip-network-interface"))
	viper.BindPFlag("cluster.domain_name", cmd.Flags().Lookup("domain-name"))
	viper.BindPFlag("cluster.kubernetes_version", cmd.Flags().Lookup("kubernetes-version"))
	viper.BindPFlag("cluster.enable_mdns", cmd.Flags().Lookup("enable-mdns"))
}

func perform(cmd *cobra.Command, args []string) {

	clusterConfig := &k8s.KubeadmConfig{}
	// Cannot use Unmarshal. Look here: https://github.com/spf13/viper/issues/368
	err := mapstructure.Decode(viper.AllSettings()["cluster"], clusterConfig)
	cobra.CheckErr(err)

	// Allow forwarding (kubeadm requirement)
	utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	cobra.CheckErr(alpine.EnsureNetFilter())

	clusterIP := net.ParseIP(clusterConfig.Ip)
	if clusterIP == nil {
		cobra.CheckErr(fmt.Errorf("ip address %v is invalid", clusterConfig.Ip))
	}

	// Check that the IP address we are targeting is bound to an interface
	ipExists, err := alpine.CheckIpExists(clusterIP)
	cobra.CheckErr(errors.Wrap(err, "While getting local ip addresses"))
	if !ipExists {
		if clusterConfig.CreateIp {
			cobra.CheckErr(alpine.AddIpAddress(clusterConfig.NetworkInterface, clusterIP))
		} else {
			cobra.CheckErr(fmt.Errorf("ip address %v is not available locally", clusterIP))
		}
	}

	// Check that the domain name is bound
	if clusterConfig.DomainName != "" {
		log.WithFields(log.Fields{
			"ip":         clusterConfig.Ip,
			"domainName": clusterConfig.DomainName,
		}).Info("Check domain name to IP mapping...")
		ips, err := net.LookupIP(clusterConfig.DomainName)
		contains := false
		if err != nil {
			ips = []net.IP{}
		} else {
			for _, existing := range ips {
				if existing.String() == clusterConfig.Ip {
					contains = true
					break
				}
			}
		}

		if !contains {
			log.WithFields(log.Fields{
				"ip":         clusterConfig.Ip,
				"domainName": clusterConfig.DomainName,
			}).Info("Mapping not found, creating...")

			cobra.CheckErr(alpine.AddIpMapping(&txeh.HostsConfig{}, clusterConfig.Ip, clusterConfig.DomainName, ips))
		}
	}

	exist := false
	restartProxy := false
	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	config, err := k8s.LoadFromDefault()
	if err == nil {
		if config.IsConfigServerAddress(clusterConfig.GetApiEndPoint()) {
			exist = true
		} else {
			// If the configuration has changed, we stop and disable the kubelet
			// that may be started and clean the configuration, i.e. delete
			// certificates and manifests.
			cobra.CheckErr(alpine.StopService(constants.KubeletServiceName))
			cobra.CheckErr(alpine.DisableService(constants.KubeletServiceName))
			cobra.CheckErr(k8s.CleanConfig())
			// Proxy may be left dangling
			restartProxy = true
		}
	} else {
		if !os.IsNotExist(err) {
			cobra.CheckErr(errors.Wrap(err, "While loading existing kubeconfig"))
		}
	}

	// Start OpenRC
	cobra.CheckErr(alpine.StartOpenRC())

	// Enable the services
	cobra.CheckErr(alpine.EnableService(constants.CrioServiceName))
	cobra.CheckErr(alpine.EnableService(constants.KubeletServiceName))
	cobra.CheckErr(alpine.StartService(constants.CrioServiceName))
	if clusterConfig.EnableMDNS {
		cobra.CheckErr(alpine.EnableService(constants.MDNSServiceName))
		cobra.CheckErr(alpine.StartService(constants.MDNSServiceName))
	}

	// CRI-O is started by OpenRC
	available, err := crio.WaitForCrio()
	cobra.CheckErr(err)
	if !available {
		log.Fatal("CRI-O not available")
	}

	if !exist {
		// Here we run kubeadm. This is done is two cases:
		// - First initialization
		// - After a configuration change. In this case, we expect kubeadm to
		//   recreate the certificates and the manifests for the control plane.
		cobra.CheckErr(k8s.RunKubeadmInit(clusterConfig))
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

	// We copy the configuration where the root user expects it
	cobra.CheckErr(config.RenameConfig(ClusterName).WriteToFile(constants.KubernetesRootConfig))

	// Perform the configuration. Not done in case it's already done
	force, err := cmd.Flags().GetBool("force-config")
	cobra.CheckErr(err)
	cobra.CheckErr(doConfiguration(clusterIP, config, force))

	log.Info("executed")
}
