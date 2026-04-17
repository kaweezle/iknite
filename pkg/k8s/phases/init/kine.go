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

// NewKineControlPlanePhase returns a new kine phase.
func NewKineControlPlanePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "kine",
		Short: "Generates the kine static Pod manifest",
		InheritFlags: []string{
			options.CfgPath,
			options.ControlPlaneEndpoint,
		},
		Run: runKineControlPlane,
	}
}

// CreateKineConfiguration writes the kine pod manifest to wr using the provided config.
func CreateKineConfiguration(wr io.Writer, config *v1alpha1.IkniteClusterSpec) error {
	if wr == nil {
		return errors.New("writer cannot be nil")
	}
	manifestTemplate, err := template.New("kine").Funcs(template.FuncMap{
		"KineImage": ikniteConfig.GetKineImage,
	}).Parse(kineManifestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse kine manifest template: %w", err)
	}

	if err := manifestTemplate.Execute(wr, config); err != nil {
		return fmt.Errorf("failed to execute kine manifest template: %w", err)
	}
	return nil
}

// WriteKineConfiguration creates the kine manifest file in manifestDir.
func WriteKineConfiguration(
	fs host.FileSystem, manifestDir string, config *v1alpha1.IkniteClusterSpec,
) (afero.File, error) {
	return writeStaticPodManifest(
		fs, manifestDir, "kine.yaml", config, CreateKineConfiguration,
	)
}

func runKineControlPlane(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return errors.New("kine phase invoked with an invalid data struct")
	}

	currentConfig := data.IkniteCluster().Spec

	_, err := WriteKineConfiguration(data.Host(), data.ManifestDir(), &currentConfig)
	return err
}
