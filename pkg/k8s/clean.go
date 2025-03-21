package k8s

// cSpell:words txeh netnsid ifname
// cSpell: disable
import (
	"fmt"
	"os"
	"syscall"

	s "github.com/bitfield/script"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
)

// cSpell: enable

func ResetIPAddress(ikniteConfig *v1alpha1.IkniteClusterSpec, isDryRun bool) error {
	if !ikniteConfig.CreateIp {
		return nil
	}

	log.WithField("isDryRun", isDryRun).Info("Resetting IP address...")
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{})
	if err != nil {
		return err
	}
	ip, err := alpine.IpMappingForHost(hosts, ikniteConfig.DomainName)
	if err != nil {
		return err
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
		return p.Error()
	}
	if !isDryRun {
		hosts.RemoveHost(ikniteConfig.DomainName)
		return hosts.Save()
	}
	return nil
}

func ResetIPTables(isDryRun bool) (err error) {
	log.WithField("isDryRun", isDryRun).Info("Cleaning up iptables rules...")
	if !isDryRun {
		_, err = s.Exec("iptables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("iptables-restore").String()
		if err != nil {
			return
		}
	}
	log.WithField("isDryRun", isDryRun).Info("Cleaning up ip6tables rules...")
	if !isDryRun {
		_, err = s.Exec("ip6tables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("ip6tables-restore").String()
	}
	return err
}

func RemoveKubeletFiles(isDryRun bool) (err error) {
	if isDryRun {
		log.Info("Would remove kubelet files in /var/lib/kubelet...")
	} else {
		log.Info("Removing kubelet files in /var/lib/kubelet...")
		_, err = s.Exec("sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'").String()
	}
	return err
}

func StopAllContainers(isDryRun bool) (err error) {
	if isDryRun {
		log.Info("Would stop all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...")
	} else {
		log.Info("Stopping all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...")
		_, err = s.Exec("/bin/sh -c 'crictl rmp -f $(crictl pods -q)'").String()
	}
	return
}

func UnmountPaths(failOnError bool, isDryRun bool) error {
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

func CleanAll(ikniteConfig *v1alpha1.IkniteClusterSpec, isDryRun bool) {

	var err error
	if err = StopAllContainers(isDryRun); err != nil {
		log.WithError(err).Warn("Error stopping all containers")
	}

	_ = UnmountPaths(false, isDryRun)

	err = RemoveKubeletFiles(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error removing kubelet files")
	}

	err = DeleteCniNamespaces(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting CNI namespaces")
	}

	err = DeleteNetworkInterfaces(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error deleting network interfaces")
	}

	log.Info("Cleaning up iptables rules...")
	err = ResetIPTables(isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error cleaning up iptables rules")
	}

	err = ResetIPAddress(ikniteConfig, isDryRun)
	if err != nil {
		log.WithError(err).Warn("Error resetting IP address")
	}
}

func processMounts(path string, remove bool, message string, isDryRun bool) error {
	fields := log.Fields{
		"path":     path,
		"remove":   remove,
		"isDryRun": isDryRun,
	}
	log.WithFields(fields).Info(message)
	logger := log.WithField("isDryRun", isDryRun)

	p := s.File("/proc/self/mounts").Column(2).Match(path).FilterLine(func(s string) string {
		logger.WithField("mount", s).Debug(message)
		if !isDryRun {
			syscall.Unmount(s, 0)
			if remove {
				os.RemoveAll(s)
			}
		}
		return s
	})
	p.Wait()
	return p.Error()
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
	return p.Error()
}

func DeleteNetworkInterfaces(isDryRun bool) error {
	prefix := ""
	if isDryRun {
		prefix = "echo "
	}
	logger := log.WithField("isDryRun", isDryRun)
	logger.Info("Deleting pods network interfaces...")
	p := s.Exec("ip -j link show").JQ(`sort_by(.ifname)| reverse | .[] | select(has("link_netnsid") or .ifname == "cni0" or .ifname == "flannel.1") | .ifname`).FilterLine(func(s string) string {
		ifname := s[1 : len(s)-1]
		command := fmt.Sprintf("%s ip link delete %s", prefix, ifname)
		logger.WithField("interface", ifname).Debugf("Deleting interface with: %s...", command)
		return command
	}).ExecForEach("{{ . }}")
	p.Wait()
	err := p.Error()
	if err != nil {
		log.WithError(err).Error("Error deleting pods network interfaces")
		return err
	} else {
		logger.Infof("Deleted pods network interfaces")
	}

	return err
}
