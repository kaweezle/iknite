package init

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

//go:embed manifests
var manifestFS embed.FS

type ManifestGeneratorFunc func(io.Writer, *v1alpha1.IkniteClusterSpec) error

type manifestData interface {
	host.HostProvider
	ManifestDirProvider
	IkniteClusterHolder
}

type PodManifestOptions struct {
	ImageFunc func() string
	Name      string
}

func NewPodManifestPhase(opts *PodManifestOptions) workflow.Phase {
	return workflow.Phase{
		Name:  opts.Name,
		Short: fmt.Sprintf("Generates the %s static Pod manifest", opts.Name),
		InheritFlags: []string{
			options.CfgPath,
			options.ControlPlaneEndpoint,
		},
		Run: func(c workflow.RunData) error {
			return runManifestPhase(c, opts)
		},
	}
}

func runManifestPhase(c workflow.RunData, opts *PodManifestOptions) error {
	data, ok := c.(manifestData)
	if !ok {
		return errors.New("control-plane phase invoked with an invalid data struct")
	}

	// Getting the cluster configuration
	currentConfig := data.IkniteCluster().Spec

	// Write the manifest configuration
	_, err := WriteStaticPodManifest(
		data.Host(),
		data.ManifestDir(),
		&currentConfig,
		opts,
	)

	return err
}

// WriteStaticPodManifest creates filename in manifestDir and writes the rendered manifest to it.
// If rendering fails, the partially-written file is removed.
func WriteStaticPodManifest(
	fs host.FileSystem,
	manifestDir string,
	config *v1alpha1.IkniteClusterSpec,
	opts *PodManifestOptions,
) (afero.File, error) {
	templateContent, err := manifestFS.ReadFile(fmt.Sprintf("manifests/%s.yaml.tmpl", opts.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s manifest template: %w", opts.Name, err)
	}
	manifestTemplate, err := template.New(opts.Name).Funcs(template.FuncMap{
		utils.CamelCase(fmt.Sprintf("%s-image", opts.Name), true): opts.ImageFunc,
	}).Parse(string(templateContent))
	if err != nil { // nocov -- Would need bad files in the embedded FS
		return nil, fmt.Errorf("failed to parse %s manifest template: %w", opts.Name, err)
	}

	if err = fs.MkdirAll(manifestDir, os.FileMode(0o755)); err != nil {
		return nil, fmt.Errorf("failed to create manifest directory %s: %w", manifestDir, err)
	}
	filename := fmt.Sprintf("%s.yaml", opts.Name)
	f, err := fs.Create(filepath.Join(manifestDir, filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", filename, err)
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		} else if closeErr == nil {
			closeErr = fs.Remove(f.Name())
			if closeErr != nil { // nocov -- mostly unreachable
				err = errors.Join(err, fmt.Errorf("while removing file %s: %w", f.Name(), closeErr))
			}
		}
	}()

	if err = manifestTemplate.Execute(f, config); err != nil { // nocov -- Would need bad files in the embedded FS
		return nil, fmt.Errorf("failed to execute %s manifest template: %w", opts.Name, err)
	}
	return f, nil
}

func NewKineControlPlanePhase() workflow.Phase {
	return NewPodManifestPhase(&PodManifestOptions{
		Name:      "kine",
		ImageFunc: ikniteConfig.GetKineImage,
	})
}

func NewKubeVipControlPlanePhase() workflow.Phase {
	return NewPodManifestPhase(&PodManifestOptions{
		Name:      "kube-vip",
		ImageFunc: ikniteConfig.GetKubeVipImage,
	})
}
