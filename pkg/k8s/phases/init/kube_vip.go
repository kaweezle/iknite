package init

import (
	"html/template"
	"io"
	"path/filepath"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
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
	template, err := template.New("config").Parse(kubeVipManifestTemplate)
	if err != nil {
		return err
	}

	return template.Execute(wr, config)
}

func WriteKubeVipConfiguration(fs afero.Fs, manifestDir string, config *v1alpha1.IkniteClusterSpec) (f afero.File, err error) {
	afs := &afero.Afero{Fs: fs}
	f, err = afs.Create(filepath.Join(manifestDir, "kube-vip.yaml"))
	if err != nil {
		return
	}
	defer f.Close()

	err = CreateKubeVipConfiguration(f, config)
	if err != nil {
		f.Close()
		afs.Remove(f.Name())
		f = nil
	}
	return
}

func runKubeVipControlPlane(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return errors.New("control-plane phase invoked with an invalid data struct")
	}

	// Getting the cluster configuration
	ikniteConfig := data.IkniteCluster().Spec

	// Write the kube-vip configuration
	_, err := WriteKubeVipConfiguration(afero.NewOsFs(), data.ManifestDir(), &ikniteConfig)

	return err
}
