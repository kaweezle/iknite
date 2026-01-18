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
// cSpell:words forbidigo
package config

// cSpell: disable
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmScheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmApiV1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	koptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	configUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	"sigs.k8s.io/yaml"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/constants"
)

// cSpell: enable

func ConfigureClusterCommand(flagSet *flag.FlagSet, ikniteConfig *v1alpha1.IkniteClusterSpec) {
	v1alpha1.SetDefaults_IkniteClusterSpec(ikniteConfig)

	flagSet.IPVar(&ikniteConfig.Ip, options.Ip, ikniteConfig.Ip, "Cluster IP address")
	flagSet.BoolVar(
		&ikniteConfig.CreateIp,
		options.IpCreate,
		ikniteConfig.CreateIp,
		"Add IP address if it doesn't exist",
	)
	flagSet.StringVar(
		&ikniteConfig.NetworkInterface,
		options.IpNetworkInterface,
		ikniteConfig.NetworkInterface,
		"Interface to which add IP",
	)
	flagSet.StringVar(&ikniteConfig.DomainName, options.DomainName, ikniteConfig.DomainName,
		"Domain name of the cluster")
	flagSet.BoolVar(&ikniteConfig.EnableMDNS, options.EnableMDNS, ikniteConfig.EnableMDNS,
		"Enable mDNS publication of domain name")

	// This flag may already be defined by kubeadm
	if flagSet.Lookup(koptions.KubernetesVersion) == nil {
		flagSet.StringVar(
			&ikniteConfig.KubernetesVersion,
			koptions.KubernetesVersion,
			ikniteConfig.KubernetesVersion,
			"Kubernetes version to install",
		)
	}
	flagSet.StringVar(
		&ikniteConfig.ClusterName,
		options.ClusterName,
		ikniteConfig.ClusterName,
		"Cluster name",
	)
	flagSet.StringVar(
		&ikniteConfig.Kustomization,
		options.Kustomization,
		ikniteConfig.Kustomization,
		"Kustomization location (URL or directory)",
	)
}

func StartPersistentPreRun(cmd *cobra.Command, _ []string) {
	flags := cmd.Flags()
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		IP,
		flags.Lookup(options.Ip),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		IPCreate,
		flags.Lookup(options.IpCreate),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		IPNetworkInterface,
		flags.Lookup(options.IpNetworkInterface),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		DomainName,
		flags.Lookup(options.DomainName),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		KubernetesVersion,
		flags.Lookup(koptions.KubernetesVersion),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		EnableMDNS,
		flags.Lookup(options.EnableMDNS),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		ClusterName,
		flags.Lookup(options.ClusterName),
	)
	//nolint:errcheck // flag exists
	_ = viper.BindPFlag(
		Kustomization,
		flags.Lookup(options.Kustomization),
	)
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
		return fmt.Errorf("while creating decoder: %w", err)
	}

	if err := decoder.Decode(viper.AllSettings()["cluster"]); err != nil {
		return fmt.Errorf("failed to decode cluster settings: %w", err)
	}
	return nil
}

// PrintIkniteConfig prints the iknite configuration in the specified format
// to the provided writer.
func PrintIkniteConfig(
	writer io.Writer,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	format string,
) error {
	output, err := MarshalIkniteConfig(ikniteConfig, format)
	if err != nil {
		return fmt.Errorf("failed to marshal iknite config: %w", err)
	}

	if _, err := writer.Write(output); err != nil {
		return fmt.Errorf("failed to write iknite config: %w", err)
	}

	// Add newline for better formatting
	if _, err := writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

func MarshalIkniteConfig(ikniteConfig *v1alpha1.IkniteClusterSpec, format string) ([]byte, error) {
	var err error
	var output []byte
	switch format {
	case "yaml", "":
		output, err = yaml.Marshal(ikniteConfig)
	case "json":
		output, err = json.MarshalIndent(ikniteConfig, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal iknite config: %w", err)
	}
	return output, nil
}

func WriteToFile(path string, data []byte) error {
	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		err = file.Close()
	}()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write data to file: %w", err)
	}

	return nil
}

func ApplyIkniteClusterSpecToClusterConfiguration(
	ikniteCfg *v1alpha1.IkniteClusterSpec,
	cfg *kubeadmApi.ClusterConfiguration,
) {
	cfg.KubernetesVersion = fmt.Sprintf("v%s", ikniteCfg.KubernetesVersion)
	if ikniteCfg.DomainName != "" {
		cfg.ControlPlaneEndpoint = ikniteCfg.DomainName
	}
}

func ApplyIkniteClusterSpecToClusterConfigurationV1(
	ikniteCfg *v1alpha1.IkniteClusterSpec,
	cfg *kubeadmApiV1.ClusterConfiguration,
) {
	cfg.KubernetesVersion = fmt.Sprintf("v%s", ikniteCfg.KubernetesVersion)
	if ikniteCfg.DomainName != "" {
		cfg.ControlPlaneEndpoint = ikniteCfg.DomainName
	}
}

// ApplyIkniteClusterSpecToInitConfiguration applies IkniteClusterSpec to InitConfiguration.
// TODO: This function should be elsewhere.
func ApplyIkniteClusterSpecToInitConfiguration(
	ikniteCfg *v1alpha1.IkniteClusterSpec,
	cfg *kubeadmApi.InitConfiguration,
) {
	ApplyIkniteClusterSpecToClusterConfiguration(ikniteCfg, &cfg.ClusterConfiguration)
	// Apply configured IP to the configuration
	ips := ikniteCfg.Ip.String()
	cfg.LocalAPIEndpoint.AdvertiseAddress = ips
	arg := &kubeadmApi.Arg{Name: "node-ip", Value: ips}
	cfg.NodeRegistration.KubeletExtraArgs = append(cfg.NodeRegistration.KubeletExtraArgs, *arg)
}

func GetKubeVipImage() string {
	return "ghcr.io/kube-vip/kube-vip:v0.8.9"
}

// GetIkniteImages returns the list of container images used by iknite.
func GetIkniteImages(ikniteConfig *v1alpha1.IkniteClusterSpec) ([]string, error) {
	// Load default kubeadm configuration to get the list of control plane images
	externalInitCfg := &kubeadmApiV1.InitConfiguration{}
	kubeadmScheme.Scheme.Default(externalInitCfg)
	externalInitCfg.SkipPhases = []string{"addon/coredns"}

	externalClusterCfg := &kubeadmApiV1.ClusterConfiguration{}
	kubeadmScheme.Scheme.Default(externalClusterCfg)
	externalClusterCfg.Networking.PodSubnet = constants.PodSubnet

	ApplyIkniteClusterSpecToClusterConfigurationV1(ikniteConfig, externalClusterCfg)

	cfg, err := configUtil.LoadOrDefaultInitConfiguration(
		"",
		externalInitCfg,
		externalClusterCfg,
		configUtil.LoadOrDefaultConfigurationOptions{
			SkipCRIDetect: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load or default init configuration: %w", err)
	}

	ApplyIkniteClusterSpecToInitConfiguration(ikniteConfig, cfg)

	containerImages := images.GetControlPlaneImages(&cfg.ClusterConfiguration)
	// Add kube vip image
	containerImages = append(containerImages, GetKubeVipImage())
	return containerImages, nil
}
