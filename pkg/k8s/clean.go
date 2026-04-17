package k8s

// cSpell:words txeh netnsid ifname
// cSpell: disable
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s/util"
)

// cSpell: enable

func ResetIPAddress(alpineHost host.Host, ikniteConfig *v1alpha1.IkniteClusterSpec, isDryRun bool) error {
	if !ikniteConfig.CreateIp {
		return nil
	}

	log.WithField("isDryRun", isDryRun).Info("Resetting IP address...")
	hosts, err := txeh.NewHosts(alpineHost.GetHostsConfig())
	if err != nil {
		return fmt.Errorf("failed to create hosts file handler: %w", err)
	}
	ip, err := alpine.IpMappingForHost(hosts, ikniteConfig.DomainName)
	if err != nil {
		log.WithField("hostname", ikniteConfig.DomainName).
			Warn("failed to get IP mapping for host:", err)
		return nil
	}
	ones, _ := ip.DefaultMask().Size()
	ipWithMask := fmt.Sprintf("%v/%d", ip, ones)

	prefix := ""
	if isDryRun {
		prefix = "echo "
	}

	p := alpineHost.ExecPipe(nil, "ip -br -4 a sh").Match(ipWithMask).Column(1).FilterLine(func(s string) string {
		log.WithField("interface", s).WithField("ip", ipWithMask).Debug("Deleting IP address...")
		return s
	})
	p = alpineHost.ExecForEach(p, fmt.Sprintf("%sip addr del %s dev {{.}}", prefix, ipWithMask))
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	if p.Error() != nil {
		return fmt.Errorf("failed to delete IP address: %w", p.Error())
	}
	if !isDryRun {
		hosts.RemoveHost(ikniteConfig.DomainName)
		if err := hosts.Save(); err != nil {
			return fmt.Errorf("failed to save hosts file: %w", err)
		}
	}
	return nil
}

func ResetIPTables(exec host.Executor, isDryRun bool) error {
	log.WithField("isDryRun", isDryRun).Info("Cleaning up iptables rules...")
	if !isDryRun {
		p := exec.ExecPipe(nil, "iptables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL")
		_, err := exec.ExecPipe(p, "iptables-restore").String()
		if err != nil {
			return fmt.Errorf("failed to clean up iptables rules: %w", err)
		}
	}
	log.WithField("isDryRun", isDryRun).Info("Cleaning up ip6tables rules...")
	if !isDryRun {
		p := exec.ExecPipe(nil, "ip6tables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL")
		_, err := exec.ExecPipe(p, "ip6tables-restore").String()
		if err != nil {
			return fmt.Errorf("failed to clean up ip6tables rules: %w", err)
		}
	}
	return nil
}

//
//nolint:lll // long command
const kubeletDataRemoveCommand = `sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'`

func RemoveKubeletFiles(exec host.Executor, isDryRun bool) error {
	if isDryRun {
		log.Info(
			"Would remove cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...",
		)
	} else {
		log.Info(
			"Removing cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...",
		)
		out, err := exec.ExecPipe(nil, kubeletDataRemoveCommand).String()
		if err != nil {
			return fmt.Errorf("failed to remove kubelet files: %s: %w", out, err)
		}
	}
	return nil
}

func StopAllContainers(exec host.Executor, isDryRun bool) error {
	if isDryRun {
		log.Info(
			"Would stop all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...",
		)
	} else {
		log.Info(
			"Stopping all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...",
		)
		_, err := exec.ExecPipe(nil, "/bin/sh -c 'crictl rmp -f $(crictl pods -q)'").String()
		if err != nil {
			return fmt.Errorf("failed to stop all containers: %w", err)
		}
	}
	return nil
}

func UnmountPaths(alpineHost host.Host, failOnError, isDryRun bool) error {
	var err error
	for _, path := range pathsToUnmount {
		err = processMounts(alpineHost, path, false, "Unmounting", isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error unmounting path")
			if failOnError {
				return err
			}
		}
	}

	for _, path := range pathsToUnmountAndRemove {
		err = processMounts(alpineHost, path, true, "Unmounting and removing", isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error unmounting and removing path")
			if failOnError {
				return err
			}
		}
	}
	return nil
}

//nolint:gocyclo // TODO: Should use a runner pattern to reduce complexity
func CleanAll(
	alpineHost host.Host,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	resetIpAddress, resetIpTables, failOnError, isDryRun bool,
) error {
	var err error
	if err = StopAllContainers(alpineHost, isDryRun); err != nil {
		log.WithError(err).Warn("Error stopping all containers")
		if failOnError {
			return err
		}
	}

	err = UnmountPaths(alpineHost, failOnError, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error unmounting paths")
		if failOnError {
			return err
		}
	}

	err = RemoveKubeletFiles(alpineHost, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error removing kubelet files")
		if failOnError {
			return err
		}
	}

	err = DeleteCniNamespaces(alpineHost, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting CNI namespaces")
		if failOnError {
			return err
		}
	}

	err = DeleteNetworkInterfaces(alpineHost, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting network interfaces")
		if failOnError {
			return err
		}
	}

	if resetIpTables {
		log.Info("Cleaning up iptables rules...")
		err = ResetIPTables(alpineHost, isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error cleaning up iptables rules")
			if failOnError {
				return err
			}
		}
	}

	if resetIpAddress {
		err = ResetIPAddress(alpineHost, ikniteConfig, isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error resetting IP address")
			if failOnError {
				return err
			}
		}
	}
	return nil
}

func processMounts(alpineHost host.Host, path string, remove bool, message string, isDryRun bool) error {
	fields := log.Fields{
		"path":     path,
		"remove":   remove,
		"isDryRun": isDryRun,
	}
	log.WithFields(fields).Info(message)
	logger := log.WithField("isDryRun", isDryRun)
	var err error
	path, err = alpineHost.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to evaluate symlinks for path %s: %w", path, err)
	}

	p := alpineHost.Pipe("/proc/self/mounts").Column(2).Match(path).FilterLine(func(s string) string {
		logger.WithField("mount", s).Debug(message)
		if !isDryRun {
			err = alpineHost.Unmount(s)
			if err != nil {
				logger.WithField("mount", s).WithError(err).Warn("Error unmounting path")
				return s
			}
			if remove {
				err = alpineHost.RemoveAll(s)
				if err != nil {
					logger.WithField("path", s).WithError(err).Warn("Error removing path")
					return s
				}
			}
		}
		return s
	})
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	if err = p.Error(); err != nil {
		return fmt.Errorf("failed to process mounts for path %s: %w", path, err)
	}
	return nil
}

func DeleteCniNamespaces(exec host.Executor, isDryRun bool) error {
	log.WithField("isDryRun", isDryRun).Info("Deleting CNI namespaces...")
	command := "ip netns delete {{.}}"
	if isDryRun {
		command = "echo " + command
	}
	logger := log.WithField("isDryRun", isDryRun)
	p := exec.ExecPipe(nil, "ip netns show").Column(1).FilterLine(func(s string) string {
		logger.WithField("namespace", s).Debug("Deleting namespace...")
		return s
	})
	p = exec.ExecForEach(p, command)
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	if err := p.Error(); err != nil {
		return fmt.Errorf("failed to delete CNI namespaces: %w", err)
	}
	return nil
}

func DeleteNetworkInterfaces(exec host.Executor, isDryRun bool) error {
	prefix := ""
	if isDryRun {
		prefix = "echo "
	}
	logger := log.WithField("isDryRun", isDryRun)
	logger.Info("Deleting pods network interfaces...")
	//nolint:lll // long string (jq pipeline)
	p := exec.ExecPipe(nil, "ip -j link show").
		JQ(`sort_by(.ifname)| reverse | .[] | select((has("link_netnsid") and .ifname != "eth0") or .ifname == "cni0" or .ifname == "flannel.1" or (.ifname | startswith("vip-"))) | .ifname`).
		FilterLine(func(s string) string {
			ifname := s[1 : len(s)-1]
			command := fmt.Sprintf("%s ip link delete %s", prefix, ifname)
			logger.WithField("interface", ifname).Debugf("Deleting interface with: %s...", command)
			return command
		})
	p = exec.ExecForEach(p, "{{ . }}")
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	err := p.Error()
	if err != nil {
		log.WithError(err).Error("Error deleting pods network interfaces")
		return fmt.Errorf("failed to delete network interfaces: %w", err)
	} else {
		logger.Infof("Deleted pods network interfaces")
	}

	return nil
}

// DeleteAPIBackendData deletes the API backend data directory.
func DeleteAPIBackendData(fs host.FileSystem, isDryRun bool, apiBackendName, apiBackendDatabaseDirectory string) error {
	apiBackendManifestPath := filepath.Join(
		kubeadmConstants.KubernetesDir,
		kubeadmConstants.ManifestsSubDirName,
		apiBackendName+".yaml",
	)
	pod, err := util.ReadStaticPodFromDisk(fs, apiBackendManifestPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to read API backend pod from disk: %w", err)
		}
		// If the API backend manifest does not exist, we assume the default data directory.
	} else {
		for i := range pod.Spec.Volumes {
			if pod.Spec.Volumes[i].Name == apiBackendName+"-data" {
				apiBackendDatabaseDirectory = pod.Spec.Volumes[i].HostPath.Path
				break
			}
		}
	}
	if isDryRun {
		log.WithField("path", apiBackendDatabaseDirectory).Info("Dry run: would delete API backend data...")
	} else {
		log.WithField("path", apiBackendDatabaseDirectory).Info("Deleting API backend data...")
		err = host.CleanDir(fs, apiBackendDatabaseDirectory)
		if err != nil {
			return fmt.Errorf("failed to delete API backend data: %w", err)
		}
	}
	return nil
}
