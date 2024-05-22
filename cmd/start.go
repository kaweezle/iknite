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
	"context"
	"os"
	"time"

	"github.com/kaweezle/iknite/cmd/options"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
	koptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
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

var ikniteConfig *v1alpha1.IkniteClusterSpec = &v1alpha1.IkniteClusterSpec{}

func init() {
	rootCmd.AddCommand(startCmd)
	flags := startCmd.Flags()

	flags.IntVarP(&timeout, options.Timeout, "t", timeout, "Wait timeout in seconds")
	configureClusterCommand(flags, ikniteConfig)
	initializeKustomization(flags)
}

func configureClusterCommand(flagSet *flag.FlagSet, ikniteConfig *v1alpha1.IkniteClusterSpec) {
	v1alpha1.SetDefaults_IkniteClusterSpec(ikniteConfig)

	flagSet.IPVar(&ikniteConfig.Ip, options.Ip, ikniteConfig.Ip, "Cluster IP address")
	flagSet.BoolVar(&ikniteConfig.CreateIp, options.IpCreate, ikniteConfig.CreateIp, "Add IP address if it doesn't exist")
	flagSet.StringVar(&ikniteConfig.NetworkInterface, options.IpNetworkInterface, ikniteConfig.NetworkInterface, "Interface to which add IP")
	flagSet.StringVar(&ikniteConfig.DomainName, options.DomainName, ikniteConfig.DomainName, "Domain name of the cluster")
	flagSet.BoolVar(&ikniteConfig.EnableMDNS, options.EnableMDNS, ikniteConfig.EnableMDNS, "Enable mDNS publication of domain name")
	flagSet.StringVar(&ikniteConfig.KubernetesVersion, koptions.KubernetesVersion, ikniteConfig.KubernetesVersion, "Kubernetes version to install")
	flagSet.StringVar(&ikniteConfig.ClusterName, options.ClusterName, ikniteConfig.ClusterName, "Cluster name")
}

func startPersistentPreRun(cmd *cobra.Command, args []string) {

	flags := cmd.Flags()
	viper.BindPFlag(config.IP, flags.Lookup(options.Ip))
	viper.BindPFlag(config.IPCreate, flags.Lookup(options.IpCreate))
	viper.BindPFlag(config.IPNetworkInterface, flags.Lookup(options.IpNetworkInterface))
	viper.BindPFlag(config.DomainName, flags.Lookup(options.DomainName))
	viper.BindPFlag(config.KubernetesVersion, flags.Lookup(koptions.KubernetesVersion))
	viper.BindPFlag(config.EnableMDNS, flags.Lookup(options.EnableMDNS))
	viper.BindPFlag(config.ClusterName, flags.Lookup(options.ClusterName))
}

// DecodeIkniteConfig decodes the configuration from the viper configuration.
// This allows providing configuration values as environment variables.
func DecodeIkniteConfig(ikniteConfig *v1alpha1.IkniteClusterSpec) error {
	// Cannot use Unmarshal. Look here: https://github.com/spf13/viper/issues/368
	decoderConfig := mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToIPHookFunc(),
		WeaklyTypedInput: true,
		Result:           ikniteConfig,
		Metadata:         nil,
	}

	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	if err != nil {
		return errors.Wrap(err, "While creating decoder")
	}

	return decoder.Decode(viper.AllSettings()["cluster"])
}

func IsIkinteReady(ctx context.Context) (bool, error) {

	cluster, err := v1alpha1.LoadIkniteCluster()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	log.WithFields(log.Fields{
		"state":   cluster.Status.State.String(),
		"phase":   cluster.Status.CurrentPhase,
		"total":   cluster.Status.WorkloadsState.Count,
		"ready":   cluster.Status.WorkloadsState.ReadyCount,
		"unready": cluster.Status.WorkloadsState.UnreadyCount,
	}).Infof(
		"status=%s, phase=%s, Workloads total=%d, ready=%d, unready=%d",
		cluster.Status.State.String(),
		cluster.Status.CurrentPhase,
		cluster.Status.WorkloadsState.Count,
		cluster.Status.WorkloadsState.ReadyCount,
		cluster.Status.WorkloadsState.UnreadyCount,
	)

	if cluster.Status.State > iknite.Initializing && cluster.Status.WorkloadsState.Count > 0 {
		return true, nil
	}

	return false, nil
}

func perform(cmd *cobra.Command, args []string) {

	cobra.CheckErr(DecodeIkniteConfig(ikniteConfig))
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
	cobra.CheckErr(alpine.StartOpenRC())

	ctx := context.Background()
	if timeout > 0 {
		err = wait.PollUntilContextTimeout(ctx, time.Second*time.Duration(2), time.Duration(timeout), true, IsIkinteReady)
	} else {
		err = wait.PollUntilContextCancel(ctx, time.Second*time.Duration(2), true, IsIkinteReady)
	}

	cobra.CheckErr(err)
	log.Info("Cluster is ready")
}
