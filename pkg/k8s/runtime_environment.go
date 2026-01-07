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

// cSpell: words tmpl netfilter txeh
// cSpell: disable
import (
	"fmt"
	"os"

	"github.com/bitfield/script"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

const (
	rcConfPreventKubeletRunning = "rc_kubelet_need=\"non-existing-service\""
)

func IsKubeletServiceRunnable(confFilePath string) (bool, error) {
	var lines int
	var err error
	if lines, err = script.File(confFilePath).Match(rcConfPreventKubeletRunning).CountLines(); err != nil {
		return false, fmt.Errorf("failed to count lines in config file: %w", err)
	}
	return lines == 0, nil
}

// PreventKubeletServiceFromStarting ensures that the kubelet service is not started
// by the OpenRC init system. It does so by adding a line to the confFilePath file.
func PreventKubeletServiceFromStarting(confFilePath string) error {
	if present, err := script.File(confFilePath).Match(rcConfPreventKubeletRunning).CountLines(); err == nil {
		if present == 0 {
			log.Info("Preventing kubelet from running")
			var lines []string
			if lines, err = script.File(confFilePath).Slice(); err == nil {
				lines = append(lines, rcConfPreventKubeletRunning)
				if _, err = script.Slice(lines).WriteFile(confFilePath); err != nil {
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
	_ = utils.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0o644)))

	if err := alpine.EnsureNetFilter(); err != nil {
		return errors.Wrap(err, "While ensuring netfilter")
	}

	// Make bridge use ip-tables
	_ = utils.WriteFile(
		"/proc/sys/net/bridge/bridge-nf-call-iptables",
		[]byte("1\n"),
		os.FileMode(int(0o644)),
	)

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
				return errors.Wrapf(err, "While adding ip address %v to interface %v",
					ikniteConfig.Ip, ikniteConfig.NetworkInterface)
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

			err := alpine.AddIpMapping(
				&txeh.HostsConfig{},
				ikniteConfig.Ip,
				ikniteConfig.DomainName,
				ips,
			) // cSpell: disable-line
			if err != nil {
				return errors.Wrapf(
					err,
					"While adding domain name %s to hosts file with ip %s",
					ikniteConfig.DomainName,
					ikniteConfig.Ip,
				)
			}
		}
	}

	log.Info("Preventing Kubelet from being started by OpenRC...")
	if err := PreventKubeletServiceFromStarting(constants.RcConfFile); err != nil {
		return errors.Wrap(err, "While preventing kubelet service from starting")
	}

	log.Info("Ensuring Iknite is launched by OpenRC...")
	if err := alpine.EnableService(constants.IkniteService); err != nil {
		return errors.Wrap(err, "While enabling iknite service")
	}

	log.Infof("Ensuring %s existence...", constants.CrictlYaml)
	if err := utils.ExecuteIfNotExist(constants.CrictlYaml, func() error {
		return utils.WriteFile(
			constants.CrictlYaml,
			[]byte("runtime-endpoint: unix://"+constants.ContainerServiceSock+"\n"),
			os.FileMode(int(0o644)))
	}); err != nil {
		return errors.Wrapf(err, "While ensuring %s existence", constants.CrictlYaml)
	}
	return nil
}
