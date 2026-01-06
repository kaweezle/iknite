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
package config

// cSpell: disable
import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	koptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
)

// cSpell: enable

func ConfigureClusterCommand(flagSet *flag.FlagSet, ikniteConfig *v1alpha1.IkniteClusterSpec) {
	v1alpha1.SetDefaults_IkniteClusterSpec(ikniteConfig)

	flagSet.IPVar(&ikniteConfig.Ip, options.Ip, ikniteConfig.Ip, "Cluster IP address")
	flagSet.BoolVar(&ikniteConfig.CreateIp, options.IpCreate, ikniteConfig.CreateIp, "Add IP address if it doesn't exist")
	flagSet.StringVar(&ikniteConfig.NetworkInterface, options.IpNetworkInterface, ikniteConfig.NetworkInterface, "Interface to which add IP")
	flagSet.StringVar(&ikniteConfig.DomainName, options.DomainName, ikniteConfig.DomainName, "Domain name of the cluster")
	flagSet.BoolVar(&ikniteConfig.EnableMDNS, options.EnableMDNS, ikniteConfig.EnableMDNS, "Enable mDNS publication of domain name")

	// This flag may already be defined by kubeadm
	if flagSet.Lookup(koptions.KubernetesVersion) == nil {
		flagSet.StringVar(&ikniteConfig.KubernetesVersion, koptions.KubernetesVersion, ikniteConfig.KubernetesVersion, "Kubernetes version to install")
	}
	flagSet.StringVar(&ikniteConfig.ClusterName, options.ClusterName, ikniteConfig.ClusterName, "Cluster name")
	flagSet.StringVar(&ikniteConfig.Kustomization, options.Kustomization, ikniteConfig.Kustomization, "Kustomization location (URL or directory)")
}

func StartPersistentPreRun(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	_ = viper.BindPFlag(IP, flags.Lookup(options.Ip))
	_ = viper.BindPFlag(IPCreate, flags.Lookup(options.IpCreate))
	_ = viper.BindPFlag(IPNetworkInterface, flags.Lookup(options.IpNetworkInterface))
	_ = viper.BindPFlag(DomainName, flags.Lookup(options.DomainName))
	_ = viper.BindPFlag(KubernetesVersion, flags.Lookup(koptions.KubernetesVersion))
	_ = viper.BindPFlag(EnableMDNS, flags.Lookup(options.EnableMDNS))
	_ = viper.BindPFlag(ClusterName, flags.Lookup(options.ClusterName))
	_ = viper.BindPFlag(Kustomization, flags.Lookup(options.Kustomization))
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

	if err := decoder.Decode(viper.AllSettings()["cluster"]); err != nil {
		return fmt.Errorf("failed to decode cluster settings: %w", err)
	}
	return nil
}
