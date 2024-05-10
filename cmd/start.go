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
	"fmt"
	"net"
	"os"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/cri"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/txn2/txeh"
)

// cSpell: enable

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Creates or starts the cluster",
	Long: `Starts the cluster. Performs the following operations:

- Starts OpenRC,
- Starts containerd,
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

	configureClusterCommand(startCmd)
	startCmd.Flags().Bool("openrc", false, "Run in OpenRC (assume OpenRC and containerd available)")
	viper.BindPFlag("openrc", startCmd.Flags().Lookup("openrc"))

	initializeKustomization(startCmd)
}

func configureClusterCommand(cmd *cobra.Command) {
	ip, err := utils.GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))
	domain_name := ""

	wsl := utils.IsOnWSL()
	if wsl {
		ip = net.ParseIP(constants.WSLIPAddress)
		domain_name = constants.WSLHostName
	}

	cmd.Flags().IP("ip", ip, "Cluster IP address")
	cmd.Flags().Bool("ip-create", wsl, "Add IP address if it doesn't exist")
	cmd.Flags().String("ip-network-interface", "eth0", "Interface to which add IP")
	cmd.Flags().String("domain-name", domain_name, "Domain name of the cluster")
	cmd.Flags().Bool("enable-mdns", wsl, "Enable mDNS publication of domain name")
	cmd.Flags().StringVar(&k8s.KubernetesVersion, "kubernetes-version", k8s.KubernetesVersion, "Kubernetes version to install")
}

func startPersistentPreRun(cmd *cobra.Command, args []string) {

	viper.BindPFlag("cluster.ip", cmd.Flags().Lookup("ip"))
	viper.BindPFlag("cluster.create_ip", cmd.Flags().Lookup("ip-create"))
	viper.BindPFlag("cluster.network_interface", cmd.Flags().Lookup("ip-network-interface"))
	viper.BindPFlag("cluster.domain_name", cmd.Flags().Lookup("domain-name"))
	viper.BindPFlag("cluster.kubernetes_version", cmd.Flags().Lookup("kubernetes-version"))
	viper.BindPFlag("cluster.enable_mdns", cmd.Flags().Lookup("enable-mdns"))
}

func PrepareKubernetesEnvironment() (*k8s.KubeadmConfig, error) {

	clusterConfig := &k8s.KubeadmConfig{}
	// Cannot use Unmarshal. Look here: https://github.com/spf13/viper/issues/368
	decoderConfig := mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToIPHookFunc(),
		WeaklyTypedInput: true,
		Result:           clusterConfig,
		Metadata:         nil,
	}

	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	cobra.CheckErr(err)

	cobra.CheckErr(decoder.Decode(viper.AllSettings()["cluster"]))

	log.WithFields(log.Fields{
		"ip":                 clusterConfig.Ip.String(),
		"kubernetes_version": clusterConfig.KubernetesVersion,
		"domain_name":        clusterConfig.DomainName,
		"create_ip":          clusterConfig.CreateIp,
		"network_interface":  clusterConfig.NetworkInterface,
		"enable_mdns":        clusterConfig.EnableMDNS,
	}).Info("Cluster configuration")

	// Allow forwarding (kubeadm requirement)
	log.Info("Ensuring basic settings...")
	utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	cobra.CheckErr(alpine.EnsureNetFilter())

	// Make bridge use ip-tables
	utils.WriteFile("/proc/sys/net/bridge/bridge-nf-call-iptables", []byte("1\n"), os.FileMode(int(0644)))

	cobra.CheckErr(alpine.EnsureMachineID())

	// Check that the IP address we are targeting is bound to an interface
	ipExists, err := alpine.CheckIpExists(clusterConfig.Ip)
	cobra.CheckErr(errors.Wrap(err, "While getting local ip addresses"))
	if !ipExists {
		if clusterConfig.CreateIp {
			cobra.CheckErr(alpine.AddIpAddress(clusterConfig.NetworkInterface, clusterConfig.Ip))
		} else {
			cobra.CheckErr(fmt.Errorf("ip address %v is not available locally", clusterConfig.Ip))
		}
	}

	// Check that the domain name is bound
	if clusterConfig.DomainName != "" {
		log.WithFields(log.Fields{
			"ip":         clusterConfig.Ip,
			"domainName": clusterConfig.DomainName,
		}).Info("Check domain name to IP mapping...")

		if contains, ips := alpine.IsHostMapped(clusterConfig.Ip, clusterConfig.DomainName); !contains {
			log.WithFields(log.Fields{
				"ip":         clusterConfig.Ip,
				"domainName": clusterConfig.DomainName,
			}).Info("Mapping not found, creating...")

			cobra.CheckErr(alpine.AddIpMapping(&txeh.HostsConfig{}, clusterConfig.Ip, clusterConfig.DomainName, ips)) // cSpell: disable-line
		}
	}
	return clusterConfig, nil
}

func perform(cmd *cobra.Command, args []string) {

	clusterConfig, err := PrepareKubernetesEnvironment()
	cobra.CheckErr(err)

	standalone := !viper.GetBool("openrc")

	exist := false
	restartProxy := false
	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	config, err := k8s.LoadFromDefault()
	if err == nil {
		if config.IsConfigServerAddress(clusterConfig.GetApiEndPoint()) {
			log.Info("Kubeconfig already exists")
			exist = true
		} else {
			// If the configuration has changed, we stop and disable the kubelet
			// that may be started and clean the configuration, i.e. delete
			// certificates and manifests.
			log.Info("Kubernetes configuration has changed. Cleaning...")
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
		log.Info("No current configuration found. Initializing...")
	}

	if standalone {
		// Start OpenRC
		log.Info("Ensuring OpenRC...")
		cobra.CheckErr(alpine.EnsureOpenRCDirectory())
		cobra.CheckErr(alpine.StartOpenRC())
		log.Info("Ensuring CRI...")
		cobra.CheckErr(alpine.EnableService(constants.ContainerServiceName))
		cobra.CheckErr(alpine.StartService(constants.ContainerServiceName))
	}

	// Enable the services
	cobra.CheckErr(alpine.EnableService(constants.KubeletServiceName))
	if clusterConfig.EnableMDNS {
		log.Info("Ensuring MDNS...")
		cobra.CheckErr(alpine.EnableService(constants.MDNSServiceName))
		cobra.CheckErr(alpine.StartService(constants.MDNSServiceName))
	}

	if standalone || !exist {
		// CRI service is started by OpenRC
		log.Info("Checking CRI is ready...")
		available, err := cri.WaitForContainerService()
		cobra.CheckErr(err)
		if !available {
			log.Fatal("CRI Service not available")
		}
	}

	if !exist {
		// Here we run kubeadm. This is done is two cases:
		// - First initialization
		// - After a configuration change. In this case, we expect kubeadm to
		//   recreate the certificates and the manifests for the control plane.
		log.WithFields(log.Fields{
			"config": clusterConfig,
		}).Info("Running kubeadm init")
		cobra.CheckErr(k8s.RunKubeadmInit(clusterConfig))
		config, err = k8s.LoadFromDefault()
		cobra.CheckErr(err)
		if restartProxy {
			log.Info("Restart kube-proxy")
			cobra.CheckErr(config.RestartProxy())
		}
		// We copy the configuration where the root user expects it
		cobra.CheckErr(config.RenameConfig(ClusterName).WriteToFile(constants.KubernetesRootConfig))
	} else if standalone {
		// The service should have been started by OpenRC
		log.Info("Checking kubelet...")
		cobra.CheckErr(config.CheckClusterRunning(10, 2, 500))
	}

	if standalone {
		// Perform the configuration. Not done in case it's already done
		force, err := cmd.Flags().GetBool("force-config")
		cobra.CheckErr(err)
		cobra.CheckErr(config.DoConfiguration(clusterConfig.Ip, force, waitTimeout))
	} else {
		log.Info("Enabling ", constants.ConfigureServiceName)
		cobra.CheckErr(alpine.EnableService(constants.ConfigureServiceName))
	}

	log.Info("executed")
}
