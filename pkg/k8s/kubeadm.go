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
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	kubernetesConfigurationDirectory = "/etc/kubernetes"
	configurationPattern             = "*.conf"
	pkiSubdirectory                  = "pki"
	manifestsSubdirectory            = "manifests"
)

var KubernetesVersion = "1.23.6"

func RunKubeadm(parameters []string) (err error) {
	log.Info("Running", "/usr/bin/kubeadm ", strings.Join(parameters, " "), "...")
	if out, err := exec.Command("/usr/bin/kubeadm", parameters...).CombinedOutput(); err != nil {
		return errors.Wrap(err, string(out))
	} else {
		log.Trace(string(out))
	}
	return
}

func RunKubeadmInit(ip net.IP) error {
	parameters := []string{
		"init",
		fmt.Sprintf("--apiserver-advertise-address=%v", ip),
		fmt.Sprintf("--kubernetes-version=%s", KubernetesVersion),
		"--pod-network-cidr=10.244.0.0/16",
		// "--control-plane-endpoint=kaweezle.local",
		"--ignore-preflight-errors=DirAvailable--var-lib-etcd,Swap",
		"--skip-phases=mark-control-plane",
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
