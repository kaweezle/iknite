// cSpell: words stretchr kyaml
package config

import (
	"bytes"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmApiV1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

func TestIkniteClusterFlagsAndDecode(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("cluster", map[string]any{
		"kubernetes_version": "1.35.1",
		"domain_name":        "iknite.local",
		"network_interface":  "eth9",
		"cluster_name":       "demo",
		"create_ip":          true,
		"enable_mdns":        true,
		"use_etcd":           true,
		"status_server_port": 11443,
		"ip":                 "10.10.10.10",
	})

	spec := &v1alpha1.IkniteClusterSpec{Ip: []byte{127, 0, 0, 1}}
	flags := pflag.NewFlagSet("cfg", pflag.ContinueOnError)
	AddIkniteClusterFlags(flags, spec)
	req.NotNil(flags.Lookup("ip"))
	req.NotNil(flags.Lookup("cluster-name"))
	req.NotNil(flags.Lookup("kustomization"))

	req.NoError(DecodeIkniteConfig(spec))
	req.Equal("1.35.1", spec.KubernetesVersion)
	req.Equal("iknite.local", spec.DomainName)
	req.Equal("eth9", spec.NetworkInterface)
	req.Equal("demo", spec.ClusterName)
	req.True(spec.CreateIp)
	req.True(spec.EnableMDNS)
	req.True(spec.UseEtcd)
}

func TestMarshalAndPrintIkniteConfig(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{ClusterName: "demo"}

	yamlOut, err := MarshalIkniteConfig(spec, "yaml")
	req.NoError(err)
	req.Contains(string(yamlOut), "clusterName: demo")

	jsonOut, err := MarshalIkniteConfig(spec, "json")
	req.NoError(err)
	req.Contains(string(jsonOut), "\"clusterName\": \"demo\"")

	_, err = MarshalIkniteConfig(spec, "xml")
	req.Error(err)

	buf := &bytes.Buffer{}
	err = PrintIkniteConfig(buf, spec, "json")
	req.NoError(err)
	req.Contains(buf.String(), "clusterName")
	req.Contains(buf.String(), "\n")
}

func TestApplyIkniteClusterSpecToKubeadmConfigs(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &v1alpha1.IkniteClusterSpec{
		KubernetesVersion: "1.35.1",
		DomainName:        "iknite.local",
		Ip:                []byte{10, 0, 0, 2},
	}

	clusterCfg := &kubeadmApi.ClusterConfiguration{}
	ApplyIkniteClusterSpecToClusterConfiguration(spec, clusterCfg)
	req.Equal("v1.35.1", clusterCfg.KubernetesVersion)
	req.Equal("iknite.local", clusterCfg.ControlPlaneEndpoint)

	clusterCfgV1 := &kubeadmApiV1.ClusterConfiguration{}
	ApplyIkniteClusterSpecToClusterConfigurationV1(spec, clusterCfgV1)
	req.Equal("v1.35.1", clusterCfgV1.KubernetesVersion)
	req.Equal("iknite.local", clusterCfgV1.ControlPlaneEndpoint)

	initCfg := &kubeadmApi.InitConfiguration{}
	ApplyIkniteClusterSpecToInitConfiguration(spec, initCfg)
	req.Equal("10.0.0.2", initCfg.LocalAPIEndpoint.AdvertiseAddress)
	req.NotEmpty(initCfg.NodeRegistration.KubeletExtraArgs)
	req.Equal("node-ip", initCfg.NodeRegistration.KubeletExtraArgs[0].Name)
	req.Equal("10.0.0.2", initCfg.NodeRegistration.KubeletExtraArgs[0].Value)
}

func TestImageHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	req.NotEmpty(GetKubeVipImage())
	req.NotEmpty(GetKineImage())

	rn, err := kyaml.Parse(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample
spec:
  template:
    spec:
      containers:
      - name: app
        image: ghcr.io/example/app:v1
      initContainers:
      - name: init
        image: ghcr.io/example/init:v1
`)
	req.NoError(err)

	images, err := GetResourceImages(rn)
	req.NoError(err)
	req.ElementsMatch([]string{"ghcr.io/example/app:v1", "ghcr.io/example/init:v1"}, images)

	value, err := GetContainerEnvVar(rn, "app", "MISSING")
	req.NoError(err)
	req.Empty(value)
}

func TestKGatewayImageParsing(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	rn, err := kyaml.Parse(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: kgateway
spec:
  template:
    spec:
      containers:
      - name: controller
        image: ghcr.io/example/controller:v1
        env:
        - name: KGW_DEFAULT_IMAGE_REGISTRY
          value: ghcr.io/custom
        - name: KGW_DEFAULT_IMAGE_TAG
          value: v9
`)
	req.NoError(err)

	image, err := getKGatewayGatewayImage(rn)
	req.NoError(err)
	req.Equal("ghcr.io/custom/envoy-wrapper:v9", image)
}
