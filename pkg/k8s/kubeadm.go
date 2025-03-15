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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bitfield/script"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/txn2/txeh"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

const (
	kubernetesConfigurationDirectory = "/etc/kubernetes"
	configurationPattern             = "*.conf"
	pkiSubdirectory                  = "pki"
	manifestsSubdirectory            = "manifests"
	rcConfFile                       = "/etc/rc.conf"
	rcConfPreventKubeletRunning      = "rc_kubelet_need=\"non-existing-service\""
)

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

func CreateKubeadmConfiguration(wr io.Writer, config *v1alpha1.IkniteClusterSpec) error {
	tmpl, err := template.New("config").Parse(kubeadmConfigTemplate)
	if err != nil {
		return err
	}

	return tmpl.Execute(wr, config)
}

func WriteKubeadmConfiguration(fs afero.Fs, config *v1alpha1.IkniteClusterSpec) (f afero.File, err error) {
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

// PreventKubeletServiceFromStarting ensures that the kubelet service is not started
// by the OpenRC init system. It does so by adding a line to the confFilePath file.
func PreventKubeletServiceFromStarting(confFilePath string) error {
	if present, err := script.File(confFilePath).Match(rcConfPreventKubeletRunning).CountLines(); err == nil {
		if present == 0 {
			log.Info("Preventing kubelet from running")
			if lines, err := script.File(confFilePath).Slice(); err == nil {
				lines = append(lines, rcConfPreventKubeletRunning)
				if _, err := script.Slice(lines).WriteFile(confFilePath); err != nil {
					return errors.Wrapf(err, "While writing %s", confFilePath)
				}
			} else {
				return errors.Wrapf(err, "While reading %s", confFilePath)
			}
		}
	} else {
		return errors.Wrapf(err, "While checking %s", confFilePath)
	}
	return nil
}

func PrepareKubernetesEnvironment(ikniteConfig *v1alpha1.IkniteClusterSpec) error {

	log.WithFields(log.Fields{
		"ip":                 ikniteConfig.Ip.String(),
		"kubernetes_version": ikniteConfig.KubernetesVersion,
		"domain_name":        ikniteConfig.DomainName,
		"create_ip":          ikniteConfig.CreateIp,
		"network_interface":  ikniteConfig.NetworkInterface,
		"enable_mdns":        ikniteConfig.EnableMDNS,
		"cluster_name":       ikniteConfig.ClusterName,
		"kustomization":      ikniteConfig.Kustomization,
	}).Info("Cluster configuration")

	// Allow forwarding (kubeadm requirement)
	log.Info("Ensuring basic settings...")
	utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	if err := alpine.EnsureNetFilter(); err != nil {
		return errors.Wrap(err, "While ensuring netfilter")
	}

	// Make bridge use ip-tables
	utils.WriteFile("/proc/sys/net/bridge/bridge-nf-call-iptables", []byte("1\n"), os.FileMode(int(0644)))

	if err := alpine.EnsureMachineID(); err != nil {
		return errors.Wrap(err, "While ensuring machine ID")
	}

	// Check that the IP address we are targeting is bound to an interface
	ipExists, err := alpine.CheckIpExists(ikniteConfig.Ip)
	if err != nil {
		return errors.Wrap(err, "While getting local ip addresses")
	}
	if !ipExists {
		if ikniteConfig.CreateIp {
			if err := alpine.AddIpAddress(ikniteConfig.NetworkInterface, ikniteConfig.Ip); err != nil {
				return errors.Wrapf(err, "While adding ip address %v to interface %v", ikniteConfig.Ip, ikniteConfig.NetworkInterface)
			}
		} else {
			return fmt.Errorf("ip address %v is not available locally", ikniteConfig.Ip)
		}
	}

	// Check that the domain name is bound
	if ikniteConfig.DomainName != "" {
		log.WithFields(log.Fields{
			"ip":         ikniteConfig.Ip,
			"domainName": ikniteConfig.DomainName,
		}).Info("Check domain name to IP mapping...")

		if contains, ips := alpine.IsHostMapped(ikniteConfig.Ip, ikniteConfig.DomainName); !contains {
			log.WithFields(log.Fields{
				"ip":         ikniteConfig.Ip,
				"domainName": ikniteConfig.DomainName,
			}).Info("Mapping not found, creating...")

			if err := alpine.AddIpMapping(&txeh.HostsConfig{}, ikniteConfig.Ip, ikniteConfig.DomainName, ips); err != nil { // cSpell: disable-line
				return errors.Wrapf(err, "While adding domain name %s to hosts file with ip %s", ikniteConfig.DomainName, ikniteConfig.Ip)
			}
		}
	}

	if err := PreventKubeletServiceFromStarting(rcConfFile); err != nil {
		return errors.Wrap(err, "While preventing kubelet service from starting")
	}
	return nil
}

func RunKubeadmInit(config *v1alpha1.IkniteClusterSpec) error {

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
	var files []string
	if files, err = filepath.Glob(configGlob); err == nil {
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
		if files, err = filepath.Glob(manifestsGlob); err == nil {
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
