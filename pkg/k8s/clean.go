package k8s

// cSpell:words txeh netnsid ifname
// cSpell: disable
import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	s "github.com/bitfield/script"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
	resetPhases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/reset"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	utilStaticPod "k8s.io/kubernetes/cmd/kubeadm/app/util/staticpod"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

// cSpell: enable

func ResetIPAddress(ikniteConfig *v1alpha1.IkniteClusterSpec, isDryRun bool) error {
	if !ikniteConfig.CreateIp {
		return nil
	}

	log.WithField("isDryRun", isDryRun).Info("Resetting IP address...")
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{})
	if err != nil {
		return fmt.Errorf("failed to create hosts file handler: %w", err)
	}
	ip, err := alpine.IpMappingForHost(hosts, ikniteConfig.DomainName)
	if err != nil {
		return fmt.Errorf("failed to get IP mapping for host: %w", err)
	}
	ones, _ := ip.DefaultMask().Size()
	ipWithMask := fmt.Sprintf("%v/%d", ip, ones)

	prefix := ""
	if isDryRun {
		prefix = "echo "
	}

	p := s.Exec("ip -br -4 a sh").Match(ipWithMask).Column(1).FilterLine(func(s string) string {
		log.WithField("interface", s).WithField("ip", ipWithMask).Debug("Deleting IP address...")
		return s
	}).ExecForEach(fmt.Sprintf("%sip addr del %s dev {{.}}", prefix, ipWithMask))
	p.Wait()
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

func ResetIPTables(isDryRun bool) error {
	log.WithField("isDryRun", isDryRun).Info("Cleaning up iptables rules...")
	if !isDryRun {
		_, err := s.Exec("iptables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL").
			Exec("iptables-restore").
			String()
		if err != nil {
			return fmt.Errorf("failed to clean up iptables rules: %w", err)
		}
	}
	log.WithField("isDryRun", isDryRun).Info("Cleaning up ip6tables rules...")
	if !isDryRun {
		_, err := s.Exec("ip6tables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL").
			Exec("ip6tables-restore").
			String()
		if err != nil {
			return fmt.Errorf("failed to clean up ip6tables rules: %w", err)
		}
	}
	return nil
}

func RemoveKubeletFiles(isDryRun bool) error {
	if isDryRun {
		log.Info(
			"Would remove cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...",
		)
	} else {
		log.Info("Removing cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...")
		out, err := s.Exec(
			"sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'").String()
		if err != nil {
			err = errors.Wrapf(err, "failed to remove kubelet files: %s", out)
		}
	}
	return nil
}

func StopAllContainers(isDryRun bool) error {
	if isDryRun {
		log.Info(
			"Would stop all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...",
		)
	} else {
		log.Info("Stopping all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...")
		_, err := s.Exec("/bin/sh -c 'crictl rmp -f $(crictl pods -q)'").String()
		if err != nil {
			return fmt.Errorf("failed to stop all containers: %w", err)
		}
	}
	return nil
}

func UnmountPaths(failOnError, isDryRun bool) error {
	var err error
	for _, path := range pathsToUnmount {
		err = processMounts(path, false, "Unmounting", isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error unmounting path")
			if failOnError {
				return err
			}
		}
	}

	for _, path := range pathsToUnmountAndRemove {
		err = processMounts(path, true, "Unmounting and removing", isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error unmounting and removing path")
			if failOnError {
				return err
			}
		}
	}
	return nil
}

func CleanAll(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	resetIpAddress, resetIpTables, failOnError, isDryRun bool,
) error {
	var err error
	if err = StopAllContainers(isDryRun); err != nil {
		log.WithError(err).Warn("Error stopping all containers")
		if failOnError {
			return err
		}
	}

	err = UnmountPaths(false, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error unmounting paths")
		if failOnError {
			return err
		}
	}

	err = RemoveKubeletFiles(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error removing kubelet files")
		if failOnError {
			return err
		}
	}

	err = DeleteCniNamespaces(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting CNI namespaces")
		if failOnError {
			return err
		}
	}

	err = DeleteNetworkInterfaces(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting network interfaces")
		if failOnError {
			return err
		}
	}

	if resetIpTables {
		log.Info("Cleaning up iptables rules...")
		err = ResetIPTables(isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error cleaning up iptables rules")
			if failOnError {
				return err
			}
		}
	}

	if resetIpAddress {
		err = ResetIPAddress(ikniteConfig, isDryRun)
		if err != nil {
			log.WithError(err).Warn("Error resetting IP address")
			if failOnError {
				return err
			}
		}
	}
	return nil
}

func processMounts(path string, remove bool, message string, isDryRun bool) error {
	fields := log.Fields{
		"path":     path,
		"remove":   remove,
		"isDryRun": isDryRun,
	}
	log.WithFields(fields).Info(message)
	logger := log.WithField("isDryRun", isDryRun)
	var err error
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to evaluate symlinks for path %s", path)
	}

	p := s.File("/proc/self/mounts").Column(2).Match(path).FilterLine(func(s string) string {
		logger.WithField("mount", s).Debug(message)
		if !isDryRun {
			_ = syscall.Unmount(s, 0)
			if remove {
				_ = os.RemoveAll(s)
			}
		}
		return s
	})
	p.Wait()
	if err = p.Error(); err != nil {
		return fmt.Errorf("failed to process mounts for path %s: %w", path, err)
	}
	return nil
}

func DeleteCniNamespaces(isDryRun bool) error {
	log.WithField("isDryRun", isDryRun).Info("Deleting CNI namespaces...")
	command := "ip netns delete {{.}}"
	if isDryRun {
		command = "echo " + command
	}
	logger := log.WithField("isDryRun", isDryRun)
	p := s.Exec("ip netns show").Column(1).FilterLine(func(s string) string {
		logger.WithField("namespace", s).Debug("Deleting namespace...")
		return s
	}).ExecForEach(command)
	p.Wait()
	if err := p.Error(); err != nil {
		return fmt.Errorf("failed to delete CNI namespaces: %w", err)
	}
	return nil
}

func DeleteNetworkInterfaces(isDryRun bool) error {
	prefix := ""
	if isDryRun {
		prefix = "echo "
	}
	logger := log.WithField("isDryRun", isDryRun)
	logger.Info("Deleting pods network interfaces...")
	//nolint:lll // long string (jq pipeline)
	p := s.Exec("ip -j link show").
		JQ(`sort_by(.ifname)| reverse | .[] | select(has("link_netnsid") or .ifname == "cni0" or .ifname == "flannel.1") | .ifname`).
		FilterLine(func(s string) string {
			ifname := s[1 : len(s)-1]
			command := fmt.Sprintf("%s ip link delete %s", prefix, ifname)
			logger.WithField("interface", ifname).Debugf("Deleting interface with: %s...", command)
			return command
		}).ExecForEach("{{ . }}")
	p.Wait()
	err := p.Error()
	if err != nil {
		log.WithError(err).Error("Error deleting pods network interfaces")
		return fmt.Errorf("failed to delete network interfaces: %w", err)
	} else {
		logger.Infof("Deleted pods network interfaces")
	}

	return nil
}

// DeleteEtcdData deletes the etcd data directory.
func DeleteEtcdData(isDryRun bool) error {
	etcdDataDir := "/var/lib/etcd"
	etcdManifestPath := filepath.Join(
		kubeadmConstants.KubernetesDir,
		kubeadmConstants.ManifestsSubDirName,
		"etcd.yaml",
	)
	etcdPod, err := utilStaticPod.ReadStaticPodFromDisk(etcdManifestPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to read etcd pod from disk")
		}
		err = nil
	} else {
		for i := range etcdPod.Spec.Volumes {
			if etcdPod.Spec.Volumes[i].Name == "etcd-data" {
				etcdDataDir = etcdPod.Spec.Volumes[i].HostPath.Path
				break
			}
		}
	}
	if isDryRun {
		log.WithField("path", etcdDataDir).Info("Dry run: would delete etcd data...")
	} else {
		log.WithField("path", etcdDataDir).Info("Deleting etcd data...")
		err = resetPhases.CleanDir(etcdDataDir)
		if err != nil {
			return fmt.Errorf("failed to delete etcd data: %w", err)
		}
	}
	return nil
}
