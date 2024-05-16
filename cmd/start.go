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
	"os"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
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

var ikniteConfig *k8s.IkniteConfig = &k8s.IkniteConfig{}

func init() {
	rootCmd.AddCommand(startCmd)

	configureClusterCommand(startCmd.Flags(), ikniteConfig)

	initializeKustomization(startCmd)
}

func configureClusterCommand(flagSet *flag.FlagSet, ikniteConfig *k8s.IkniteConfig) {
	k8s.SetDefaults_IkniteConfig(ikniteConfig)

	flagSet.IPVar(&ikniteConfig.Ip, "ip", ikniteConfig.Ip, "Cluster IP address")
	flagSet.BoolVar(&ikniteConfig.CreateIp, "ip-create", ikniteConfig.CreateIp, "Add IP address if it doesn't exist")
	flagSet.StringVar(&ikniteConfig.NetworkInterface, "ip-network-interface", ikniteConfig.NetworkInterface, "Interface to which add IP")
	flagSet.StringVar(&ikniteConfig.DomainName, "domain-name", ikniteConfig.DomainName, "Domain name of the cluster")
	flagSet.BoolVar(&ikniteConfig.EnableMDNS, "enable-mdns", ikniteConfig.EnableMDNS, "Enable mDNS publication of domain name")
	flagSet.StringVar(&ikniteConfig.KubernetesVersion, "kubernetes-version", ikniteConfig.KubernetesVersion, "Kubernetes version to install")
	flagSet.StringVar(&ikniteConfig.ClusterName, "cluster-name", ikniteConfig.ClusterName, "Cluster name")
}

func startPersistentPreRun(cmd *cobra.Command, args []string) {

	viper.BindPFlag("cluster.ip", cmd.Flags().Lookup("ip"))
	viper.BindPFlag("cluster.create_ip", cmd.Flags().Lookup("ip-create"))
	viper.BindPFlag("cluster.network_interface", cmd.Flags().Lookup("ip-network-interface"))
	viper.BindPFlag("cluster.domain_name", cmd.Flags().Lookup("domain-name"))
	viper.BindPFlag("cluster.kubernetes_version", cmd.Flags().Lookup("kubernetes-version"))
	viper.BindPFlag("cluster.enable_mdns", cmd.Flags().Lookup("enable-mdns"))
	viper.BindPFlag("cluster.cluster_name", cmd.Flags().Lookup("cluster-name"))
}

func perform(cmd *cobra.Command, args []string) {

	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ikniteConfig))

	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	config, err := k8s.LoadFromDefault()
	if err == nil {
		if config.IsConfigServerAddress(ikniteConfig.GetApiEndPoint()) {
			log.Info("Kubeconfig already exists")
		} else {
			// If the configuration has changed, we stop and disable the kubelet
			// that may be started and clean the configuration, i.e. delete
			// certificates and manifests.
			log.Info("Kubernetes configuration has changed. Cleaning...")
			cobra.CheckErr(alpine.StopService(constants.IkniteService))
			cobra.CheckErr(k8s.CleanConfig())
		}
	} else {
		if !os.IsNotExist(err) {
			cobra.CheckErr(errors.Wrap(err, "While loading existing kubeconfig"))
		}
		log.Info("No current configuration found. Initializing...")
	}

	// Start OpenRC
	log.Info("Ensuring Iknite...")
	cobra.CheckErr(alpine.EnableService(constants.IkniteService))
	log.Info("Ensuring OpenRC...")
	cobra.CheckErr(alpine.EnsureOpenRCDirectory())
	cobra.CheckErr(alpine.StartOpenRC())
	cobra.CheckErr(alpine.StartService(constants.IkniteService))

	log.Info("executed")
}
