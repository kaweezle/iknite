package init //nolint:dupl // kube-vip and kine intentionally share the same static-pod-manifest phase structure

import (
	"errors"
	"fmt"
	"html/template"
	"io"

	"github.com/spf13/afero"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
)

func NewKubeVipControlPlanePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "kube-vip",
		Short: "Generates the kube-vip static Pod manifest",
		InheritFlags: []string{
			options.CfgPath,
			options.ControlPlaneEndpoint,
		},
		Run: runKubeVipControlPlane,
	}
}

func CreateKubeVipConfiguration(wr io.Writer, config *v1alpha1.IkniteClusterSpec) error {
	if wr == nil {
		return errors.New("writer cannot be nil")
	}
	manifestTemplate, err := template.New("config").Funcs(template.FuncMap{
		"KubeVipImage": ikniteConfig.GetKubeVipImage,
	}).Parse(kubeVipManifestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse kube-vip manifest template: %w", err)
	}

	if err := manifestTemplate.Execute(wr, config); err != nil {
		return fmt.Errorf("failed to execute kube-vip manifest template: %w", err)
	}
	return nil
}

// WriteKubeVipConfiguration creates the kube-vip manifest file in manifestDir.
func WriteKubeVipConfiguration(
	fs host.FileSystem, manifestDir string, config *v1alpha1.IkniteClusterSpec,
) (afero.File, error) {
	return writeStaticPodManifest(
		fs, manifestDir, "kube-vip.yaml", config, CreateKubeVipConfiguration,
	)
}

func runKubeVipControlPlane(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return errors.New("control-plane phase invoked with an invalid data struct")
	}

	// Getting the cluster configuration
	currentConfig := data.IkniteCluster().Spec

	// Write the kube-vip configuration
	_, err := WriteKubeVipConfiguration(data.Host(), data.ManifestDir(), &currentConfig)

	return err
}
