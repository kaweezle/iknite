package k8s

// cSpell:words txeh
// cSpell: disable
import (
	"fmt"
	"strings"

	s "github.com/bitfield/script"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
)

// cSpell: enable

func ResetIPAddress(ikniteConfig *v1alpha1.IkniteClusterSpec) error {
	if !ikniteConfig.CreateIp {
		return nil
	}

	log.Info("Resetting IP address...")
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

	p := s.Exec("ip -br -4 a sh").Match(ipWithMask).Column(1).FilterLine(func(s string) string {
		log.WithField("interface", s).WithField("ip", ipWithMask).Debug("Deleting IP address...")
		return s
	}).ExecForEach(fmt.Sprintf("ip addr del %s dev {{.}}", ipWithMask))
	p.Wait()
	if p.Error() != nil {
		return p.Error()
	}
	hosts.RemoveHost(ikniteConfig.DomainName)
	return hosts.Save()
}

func ResetIPTables() error {
	log.Info("Cleaning up iptables rules...")
	_, err := s.Exec("iptables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("iptables-restore").String()
	if err != nil {
		return err
	}
	log.Info("Cleaning up ip6tables rules...")
	_, err = s.Exec("ip6tables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("ip6tables-restore").String()
	return err
}

func RemoveKubeletFiles() error {
	log.Info("Removing kubelet files in /var/lib/kubelet...")
	_, err := s.Exec("sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'").String()
	return err
}

func StopAllContainers() error {
	log.Info("Stopping all containers...")
	_, err := s.Exec("/bin/zsh -c 'export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock;crictl rmp -f $(crictl pods -q)'").String()
	return err
}

func UnmountPaths(failOnError bool) error {
	var err error
	for _, path := range pathsToUnmount {
		err = doUnmount(path)
		if err != nil {
			log.WithError(err).Warn("Error unmounting path")
			if failOnError {
				return err
			}
		}
	}

	for _, path := range pathsToUnmountAndRemove {
		err = doUnmountAndRemove(path)
		if err != nil {
			log.WithError(err).Warn("Error unmounting and removing path")
			if failOnError {
				return err
			}
		}
	}
	return nil
}

func CleanAll(ikniteConfig *v1alpha1.IkniteClusterSpec) {

	var err error
	if err = StopAllContainers(); err != nil {
		log.WithError(err).Warn("Error stopping all containers")
	}

	_ = UnmountPaths(false)

	err = RemoveKubeletFiles()
	if err != nil {
		log.WithError(err).Warn("Error removing kubelet files")
	}

	err = DeleteCniNamespaces()
	if err != nil {
		log.WithError(err).Warn("Error deleting CNI namespaces")
	}

	err = DeleteNetworkInterfaces()
	if err != nil {
		log.WithError(err).Warn("Error deleting network interfaces")
	}

	log.Info("Cleaning up iptables rules...")
	_, err = s.Exec("iptables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("iptables-restore").String()
	if err != nil {
		log.WithError(err).Warn("Error cleaning up iptables rules")
	}

	log.Info("Cleaning up ip6tables rules...")
	_, err = s.Exec("ip6tables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("ip6tables-restore").String()
	if err != nil {
		log.WithError(err).Warn("Error cleaning up ip6tables rules")
	}

	err = ResetIPAddress(ikniteConfig)
	if err != nil {
		log.WithError(err).Warn("Error resetting IP address")
	}
}

func processMounts(path string, command string, message string) error {
	fields := log.Fields{
		"path":    path,
		"command": command,
	}
	log.WithFields(fields).Info(message)

	p := s.File("/proc/self/mounts").Column(2).Match(path).FilterLine(func(s string) string {
		log.WithField("mount", s).Debug(message)
		return s
	}).ExecForEach(command)
	p.Wait()
	return p.Error()
}

func doUnmountAndRemove(path string) error {
	return processMounts(path, "sh -c 'umount \"{{.}}\" && rm -rf \"{{.}}\"'", "Unmounting and removing")
}

func doUnmount(path string) error {
	return processMounts(path, "umount {{.}}", "Unmounting")
}

func DeleteCniNamespaces() error {
	log.Info("Deleting CNI namespaces...")
	p := s.Exec("ip netns show").Column(1).FilterLine(func(s string) string {
		log.WithField("namespace", s).Debug("Deleting namespace...")
		return s
	}).ExecForEach("ip netns delete {{.}}")
	p.Wait()
	return p.Error()
}

func DeleteNetworkInterfaces() error {
	log.Info("Deleting pods network interfaces...")
	p := s.Exec("ip link show").Match("master cni0").Column(2).FilterLine(func(s string) string {
		result := strings.Split(s, "@")[0]
		log.WithField("interface", result).Debug("Deleting interface...")
		return result
	}).ExecForEach("ip link delete {{.}}")
	p.Wait()
	err := p.Error()
	if err != nil {
		log.WithError(err).Error("Error deleting pods network interfaces")
		return err
	} else {
		log.Infof("Deleted pods network interfaces")
	}

	log.Info("Deleting cni0 network interface...")
	if _, err = s.Exec("ip link show").Match("cni0").ExecForEach("ip link delete cni0").Stdout(); err != nil {
		log.WithError(err).Error("Error deleting cni0 network interface")
		return err
	}

	log.Info("Deleting flannel.1 network interface...")
	_, err = s.Exec("ip link show").Match("flannel.1").ExecForEach("ip link delete flannel.1").Stdout()
	return err
}
