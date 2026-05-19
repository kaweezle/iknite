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

// cSpell: words tmpl netfilter cpuset procs lithammer
import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lithammer/dedent"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

const (
	RcConfPreventServiceRunning = "rc_%s_need=\"non-existing-service\""
	RcConfIkniteNeedsNetworking = "rc_iknite_need=\"networking\""
)

// HasConfigFileConfigurationLine checks if the specified configuration line is present in the given configuration file.
// It returns true if the line is present, false otherwise.
func HasConfigFileConfigurationLine(fs host.FileSystem, confFilePath, line string) (bool, error) {
	var lines int
	var err error
	if lines, err = fs.Pipe(confFilePath).
		Match(line).
		CountLines(); err != nil {
		return false, fmt.Errorf("failed to count lines in config file: %w", err)
	}
	return lines > 0, nil
}

// EnsureConfigFileHasConfigurationLine ensures that the specified configuration line is present in the given
// configuration file. If the line is already present, it does nothing. If the line is not present, it adds it to the
// end of the file.
func EnsureConfigFileHasConfigurationLine(
	fs host.FileSystem,
	confFilePath, line string,
	logger *slog.Logger,
) error {
	present, err := fs.Pipe(confFilePath).Match(line).CountLines()
	if err != nil {
		return fmt.Errorf("while checking %s for %s: %w", confFilePath, line, err)
	}
	if present > 0 {
		logger.Info("Configuration line is already present", "line", line, "confFilePath", confFilePath)
		return nil
	}
	logger.Info("Adding configuration line", "line", line, "confFilePath", confFilePath)
	var lines []string
	if lines, err = fs.Pipe(confFilePath).Slice(); err == nil {
		lines = append(lines, line)
		content := strings.Join(lines, "\n") + "\n"
		if err = fs.WriteFile(confFilePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("while writing %s: %w", confFilePath, err)
		}
	} else {
		return fmt.Errorf("while reading %s: %w", confFilePath, err)
	}
	return nil
}

// IsServiceRunnable checks if the kubelet service is allowed to be started by the OpenRC init system.
// It does so by checking if the confFilePath file contains the line that prevents kubelet from being started.
func IsServiceRunnable(fs host.FileSystem, confFilePath, serviceName string) (bool, error) {
	line := fmt.Sprintf(RcConfPreventServiceRunning, serviceName)
	result, err := HasConfigFileConfigurationLine(fs, confFilePath, line)
	if err != nil {
		return result, err
	}
	return !result, err
}

// PreventServiceFromStarting ensures that the kubelet service is not started
// by the OpenRC init system. It does so by adding a line to the confFilePath file.
func PreventServiceFromStarting(fs host.FileSystem, confFilePath, serviceName string, logger *slog.Logger) error {
	line := fmt.Sprintf(RcConfPreventServiceRunning, serviceName)
	return EnsureConfigFileHasConfigurationLine(fs, confFilePath, line, logger)
}

// MakeIkniteServiceNeedNetworking ensures that the iknite OpenRC service has a dependency on the networking service, by
//
//	adding a line to the confFilePath file.
func MakeIkniteServiceNeedNetworking(fs host.FileSystem, confFilePath string, logger *slog.Logger) error {
	return EnsureConfigFileHasConfigurationLine(fs, confFilePath, RcConfIkniteNeedsNetworking, logger)
}

// EnsureNetworkInterfacesConfiguration ensures that the /etc/network/interfaces file exists.
func EnsureNetworkInterfacesConfiguration(fs host.FileSystem, logger *slog.Logger) error {
	logger.Info("Ensuring network interfaces configuration", "file", constants.NetworkInterfacesConfFile)
	if err := host.ExecuteIfNotExist(fs, constants.NetworkInterfacesConfFile, func() error {
		return fs.WriteFile(
			constants.NetworkInterfacesConfFile,
			[]byte(dedent.Dedent(`
                # cSpell: words iface
                auto lo
                iface lo inet loopback

                auto eth0
                iface eth0
                  use dhcp
            `)),
			os.FileMode(int(0o644)),
		)
	}); err != nil {
		return fmt.Errorf("while ensuring network interfaces configuration: %w", err)
	}
	return nil
}

// ensureIpConfiguration checks if an outbound IP is available and configures the cluster IP accordingly.
func ensureIpConfiguration(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	alpineHost host.Host,
	logger *slog.Logger,
) error {
	_, err := alpineHost.GetOutboundIP()
	if err != nil {
		logger.Warn("Could not get current IP", utils.ErrorKey, err)
		if err = MakeIkniteServiceNeedNetworking(alpineHost, constants.RcConfFile, logger); err != nil {
			return fmt.Errorf("while making iknite service need networking: %w", err)
		}
		return nil
	}

	// Check that the IP address we are targeting is bound to an interface
	ipExists, err := alpineHost.CheckIpExists(ikniteConfig.Ip)
	if err != nil {
		return fmt.Errorf("while getting local ip addresses: %w", err)
	}
	if !ipExists {
		if ikniteConfig.CreateIp {
			if err := alpine.AddIpAddress(alpineHost, ikniteConfig.NetworkInterface,
				ikniteConfig.Ip, logger); err != nil {
				return fmt.Errorf("while adding ip address %v to interface %v: %w",
					ikniteConfig.Ip, ikniteConfig.NetworkInterface, err)
			}
		} else {
			return fmt.Errorf("ip address %v is not available locally", ikniteConfig.Ip)
		}
	}
	return nil
}

// EnableCGroupSubtreeControl enables cgroup subtree control if it is not already enabled. This is required for kubelet
// to be able to manage cgroups properly.
// It does so by creating a new cgroup, moving all current processes to it, and then enabling subtree control for all
// controllers.
// We assume that we are running in a privileged container and that we have access to the cgroup filesystem. If this is
// not the case, this function will fail. We also assume that we are running with CGroups V2.
func EnableCGroupSubtreeControl(fs host.FileSystem, logger *slog.Logger) error {
	// Check if subtree control is already enabled
	content, err := fs.ReadFile("/sys/fs/cgroup/cgroup.subtree_control")
	if err != nil {
		return fmt.Errorf("while reading cgroup.subtree_control: %w", err)
	}
	if strings.Contains(string(content), "cpuset") {
		logger.Info("CGroup subtree control is already enabled")
		return nil
	}

	logger.Info("Enabling cgroup subtree control...")
	// Create a group to move all current processes to, as enabling subtree control requires that no processes are in
	// the root cgroup.
	err = fs.MkdirAll("/sys/fs/cgroup/iknite_init", 0o755)
	if err != nil {
		return fmt.Errorf("while creating cgroup directory: %w", err)
	}
	// Move all processes to the new group
	if processNumbers, procErr := fs.Pipe("/sys/fs/cgroup/cgroup.procs").Slice(); procErr == nil {
		for _, processNumber := range processNumbers {
			if procErr = fs.WriteFile(
				"/sys/fs/cgroup/iknite_init/cgroup.procs",
				[]byte(processNumber),
				0o644,
			); procErr != nil {
				// Not sure if we should return an error here or just log it and continue, as some processes might have
				// ended between the time we read the process numbers and now. For now, let's log it and continue.
				logger.Warn(
					"While moving process to iknite_init cgroup",
					utils.ErrorKey,
					procErr,
					"processNumber",
					processNumber,
				)
			}
		}
	} else {
		return fmt.Errorf("while reading cgroup.procs: %w", procErr)
	}

	// Now read the current controllers and create the string to enable all of them in the subtree control
	controllersContent, err := fs.ReadFile("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		return fmt.Errorf("while reading cgroup.controllers: %w", err)
	}

	controllers := strings.Fields(string(controllersContent))
	var enableControllers strings.Builder
	for _, controller := range controllers {
		enableControllers.WriteString("+" + controller + " ")
	}

	// Enable subtree control
	err = fs.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte(enableControllers.String()+"\n"), 0o644)
	if err != nil {
		return fmt.Errorf("while enabling cgroup subtree control: %w", err)
	}
	logger.Info("CGroup subtree control enabled")
	return nil
}

// PrepareKubernetesEnvironment prepares the environment for running Kubernetes by ensuring that necessary configuration
// files are in place and that the iknite service is properly configured to depend on the networking service. It also
// checks that the target IP address is available and adds it if necessary.
//

func PrepareKubernetesEnvironment(
	ctx context.Context,
	alpineHost host.Host,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
) error {
	logger := util.LoggerFromContext(ctx)
	logger.Info("Cluster configuration", "ip", ikniteConfig.Ip.String(),
		"kubernetes_version", ikniteConfig.KubernetesVersion,
		"domain_name", ikniteConfig.DomainName,
		"create_ip", ikniteConfig.CreateIp,
		"network_interface", ikniteConfig.NetworkInterface,
		"enable_mdns", ikniteConfig.EnableMDNS,
		"cluster_name", ikniteConfig.ClusterName,
		"kustomization", ikniteConfig.Kustomization,
	)

	// Allow forwarding (kubeadm requirement)
	logger.Info("Ensuring basic settings...")
	err := alpineHost.WriteFile(
		"/proc/sys/net/ipv4/ip_forward",
		[]byte("1\n"),
		os.FileMode(int(0o644)),
	)
	if err != nil {
		logger.Info("Could not write to /proc/sys/net/ipv4/ip_forward", utils.ErrorKey, err)
	}

	if err = alpine.EnsureNetFilter(alpineHost, logger); err != nil {
		return fmt.Errorf("while ensuring netfilter: %w", err)
	}

	// Make bridge use ip-tables
	err = alpineHost.WriteFile(
		"/proc/sys/net/bridge/bridge-nf-call-iptables",
		[]byte("1\n"),
		os.FileMode(int(0o644)),
	)
	if err != nil {
		logger.Warn("While enabling bridge-nf-call-iptables", utils.ErrorKey, err)
	}

	// Setting loose mode on reverse path forwarding because of VIP addresses
	err = alpineHost.WriteFile(
		"/proc/sys/net/ipv4/conf/default/rp_filter",
		[]byte("2\n"),
		os.FileMode(int(0o644)),
	)
	if err != nil {
		logger.Info("While enabling loose mode on reverse path forwarding (rp_filter=2)", utils.ErrorKey, err)
	}

	if err = EnableCGroupSubtreeControl(alpineHost, logger); err != nil {
		return fmt.Errorf("while enabling cgroup subtree control: %w", err)
	}

	if err = alpine.EnsureMachineID(alpineHost, logger); err != nil {
		return fmt.Errorf("while ensuring machine ID: %w", err)
	}

	if err := ensureIpConfiguration(ikniteConfig, alpineHost, logger); err != nil {
		return fmt.Errorf("while ensuring IP configuration: %w", err)
	}

	// Check that the domain name is bound
	if ikniteConfig.DomainName != "" {
		logger.Info("Check domain name to IP mapping...", "ip", ikniteConfig.Ip, "domainName", ikniteConfig.DomainName)

		if contains, ips := alpineHost.IsHostMapped(
			ctx,
			ikniteConfig.Ip,
			ikniteConfig.DomainName,
		); !contains {
			logger.Info("Mapping not found, creating...", "ip", ikniteConfig.Ip, "domainName", ikniteConfig.DomainName)

			err := alpine.AddIpMapping(
				alpineHost.GetHostsConfig(),
				ikniteConfig.Ip,
				ikniteConfig.DomainName,
				ips,
			) // cSpell: disable-line
			if err != nil {
				return fmt.Errorf(
					"while adding domain name %s to hosts file with ip %s: %w",
					ikniteConfig.DomainName,
					ikniteConfig.Ip,
					err,
				)
			}
		}
	}

	logger.Info("Preventing Kubelet from being started by OpenRC...")
	if err := PreventServiceFromStarting(alpineHost, constants.RcConfFile, KubeletName, logger); err != nil {
		return fmt.Errorf("while preventing kubelet service from starting: %w", err)
	}

	logger.Info("Ensuring Iknite is launched by OpenRC...")
	if err := alpine.EnableService(alpineHost, constants.IkniteService); err != nil {
		return fmt.Errorf("while enabling iknite service: %w", err)
	}

	logger.Info("Ensuring file existence...", "file", constants.CrictlYaml)
	if err := host.ExecuteIfNotExist(alpineHost, constants.CrictlYaml, func() error {
		return alpineHost.WriteFile(
			constants.CrictlYaml,
			[]byte("runtime-endpoint: unix://"+constants.ContainerServiceSock+"\n"),
			os.FileMode(int(0o644)))
	}); err != nil {
		return fmt.Errorf("while ensuring %s existence: %w", constants.CrictlYaml, err)
	}
	return nil
}
