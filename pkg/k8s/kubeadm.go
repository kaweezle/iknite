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
package k8s

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

const (
	kubernetesConfigurationDirectory = "/etc/kubernetes"
	configurationPattern             = "*.conf"
	pkiSubdirectory                  = "pki"
	manifestsSubdirectory            = "manifests"
)

var KubernetesVersion = "1.28.4"

type KubeadmConfig struct {
	Ip                string `mapstructure:"ip"`
	KubernetesVersion string `mapstructure:"kubernetes_version"`
	DomainName        string `mapstructure:"domain_name"`
	CreateIp          bool   `mapstructure:"create_ip"`
	NetworkInterface  string `mapstructure:"network_interface"`
	EnableMDNS        bool   `mapstructure:"enable_mdns"`
}

func (c *KubeadmConfig) GetApiEndPoint() string {
	if c.DomainName != "" {
		return c.DomainName
	}
	return c.Ip
}

const kubeadmConfigTemplate = `
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: "{{ .KubernetesVersion }}"
networking:
  podSubnet: 10.244.0.0/16
{{- if .DomainName }}
controlPlaneEndpoint: {{ .DomainName }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: {{ .Ip }}
skipPhases:
  - mark-control-plane
nodeRegistration:
{{- if .DomainName }}
  name: {{ .DomainName }}
{{- end }}
  kubeletExtraArgs:
    node-ip: {{ .Ip }}
  ignorePreflightErrors:
    - DirAvailable--var-lib-etcd
    - Swap
`

func CreateKubeadmConfiguration(wr io.Writer, config *KubeadmConfig) error {
	tmpl, err := template.New("config").Parse(kubeadmConfigTemplate)
	if err != nil {
		return err
	}

	return tmpl.Execute(wr, config)
}

func WriteKubeadmConfiguration(fs afero.Fs, config *KubeadmConfig) (f afero.File, err error) {
	afs := &afero.Afero{Fs: fs}
	f, err = afs.TempFile("", "config*.yaml")
	if err != nil {
		return
	}
	defer f.Close()

	err = CreateKubeadmConfiguration(f, config)
	if err != nil {
		f.Close()
		afs.Remove(f.Name())
		f = nil
	}
	return
}

func RunKubeadm(parameters []string) (err error) {
	log.Info("Running", "/usr/bin/kubeadm ", strings.Join(parameters, " "), "...")
	if out, err := utils.Exec.Run(true, "/usr/bin/kubeadm", parameters...); err != nil {
		return errors.Wrap(err, string(out))
	} else {
		log.Trace(string(out))
	}
	return
}

func RunKubeadmInit(config *KubeadmConfig) error {

	fs := afero.NewOsFs()
	f, err := WriteKubeadmConfiguration(fs, config)
	if err != nil {
		return err
	}

	defer fs.Remove(f.Name())
	parameters := []string{
		"init",
		"--config",
		f.Name(),
	}
	return RunKubeadm(parameters)
}

func CleanConfig() (err error) {
	log.
		WithField("dir", kubernetesConfigurationDirectory).
		Info("Cleaning Kubernetes configuration directory")
	configGlob := path.Join(kubernetesConfigurationDirectory, configurationPattern)
	if files, err := filepath.Glob(configGlob); err == nil {
		for _, file := range files {
			log.WithField("file", file).Trace("Removing configuration file")
			err = os.Remove(file)
			if err != nil {
				errors.WithMessagef(err, "While removing configuration file %s", file)
				break
			}
		}
	}

	if err == nil {
		manifestsGlob := path.Join(kubernetesConfigurationDirectory, manifestsSubdirectory, "*.yaml")
		if files, err := filepath.Glob(manifestsGlob); err == nil {
			for _, file := range files {
				log.WithField("file", file).Trace("Removing manifest file")
				err = os.Remove(file)
				if err != nil {
					errors.WithMessagef(err, "While removing manifest file %s", file)
					break
				}
			}
		}
	}

	if err == nil {
		certsDir := path.Join(kubernetesConfigurationDirectory, pkiSubdirectory)
		err = utils.RemoveDirectoryContents(certsDir, func(file string) (result bool) {
			result = file != "ca.key" && file != "ca.crt"
			log.WithFields(log.Fields{
				"file":   filepath.Join(certsDir, file),
				"delete": result,
			}).Trace("Removing certificate file or directory")
			return
		})
	}

	return
}
