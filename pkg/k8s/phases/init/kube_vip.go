package init

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
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

func WriteKubeVipConfiguration(
	fs afero.Fs, manifestDir string, config *v1alpha1.IkniteClusterSpec,
) (afero.File, error) {
	afs := &afero.Afero{Fs: fs}
	f, err := afs.Create(filepath.Join(manifestDir, "kube-vip.yaml"))
	if err != nil {
		return f, fmt.Errorf("failed to create kube-vip.yaml file: %w", err)
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		} else if closeErr == nil {
			closeErr = afs.Remove(f.Name())
			if closeErr != nil {
				err = errors.Join(err, fmt.Errorf("while removing file %s: %w", f.Name(), closeErr))
			}
		}
	}()

	err = CreateKubeVipConfiguration(f, config)
	return f, err
}

func runKubeVipControlPlane(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return errors.New("control-plane phase invoked with an invalid data struct")
	}

	// Getting the cluster configuration
	currentConfig := data.IkniteCluster().Spec

	// Write the kube-vip configuration
	_, err := WriteKubeVipConfiguration(afero.NewOsFs(), data.ManifestDir(), &currentConfig)

	return err
}
