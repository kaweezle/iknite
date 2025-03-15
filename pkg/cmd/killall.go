package cmd

import (
	"fmt"
	"os"
	"strings"

	s "github.com/bitfield/script"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/constants"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/txn2/txeh"
)

var (
	stopServices   = true
	stopContainers = true
	unmountPaths   = true
	resetCni       = true
	resetIptables  = true
	resetKubelet   = false
	resetIpAddress = false

	pathsToUnmount          = []string{"/var/lib/kubelet/pods", "/var/lib/kubelet/plugins", "/var/lib/kubelet"}
	pathsToUnmountAndRemove = []string{"/run/containerd", "/run/netns", "/run/ipcns", "/run/utsns"}
)

const ()

var killallCmd = &cobra.Command{
	Use:   "killall",
	Short: "Kill the cluster and clean up the environment",
	Long: `Kill the cluster and clean up the environment.

This command stops all the services and removes the configuration files. It also
removes the network interfaces and the IP address assigned to the cluster.

This command must be run as root.

`,
	Run: performKillall,
}

func initializeKillall(flags *flag.FlagSet) {
	flags.BoolVar(&stopServices, options.StopServices, stopServices, "Stop the services")
	flags.BoolVar(&stopContainers, options.StopContainers, stopContainers, "Stop containers")
	flags.BoolVar(&unmountPaths, options.UnmountPaths, unmountPaths, "Unmount paths")
	flags.BoolVar(&resetCni, options.ResetCNI, resetCni, "Reset CNI")
	flags.BoolVar(&resetIptables, options.ResetIPTables, resetIptables, "Reset iptables")
	flags.BoolVar(&resetKubelet, options.ResetKubelet, resetKubelet, "Reset kubelet")
	flags.BoolVar(&resetIpAddress, options.ResetIPAddress, resetIpAddress, "Reset IP address")
}

func init() {
	rootCmd.AddCommand(killallCmd)

	initializeKillall(killallCmd.Flags())
	configureClusterCommand(killallCmd.Flags(), ikniteConfig)
}

func performKillall(cmd *cobra.Command, args []string) {

	if resetKubelet {
		log.Info("Resetting kubelet...")
		_, err := s.Exec("/usr/bin/kubeadm reset --force").Stdout()
		cobra.CheckErr(err)
		os.Remove(constants.KubernetesRootConfig)
	}

	if stopServices {
		log.Infof("Stopping %s...", constants.IkniteService)
		cobra.CheckErr(alpine.StopService(constants.IkniteService))

		if stopContainers {
			log.Info("Stopping all containers...")
			if _, err := s.Exec("/bin/zsh -c 'export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock;crictl rmp -f $(crictl pods -q)'").String(); err != nil {
				log.WithError(err).Warn("Error stopping all containers")
			}
		}

		log.Infof("Stopping %s...", constants.ContainerServiceName)
		cobra.CheckErr(alpine.StopService(constants.ContainerServiceName))
	}

	if unmountPaths {
		for _, path := range pathsToUnmount {
			cobra.CheckErr(doUnmount(path))
		}

		for _, path := range pathsToUnmountAndRemove {
			cobra.CheckErr(doUnmountAndRemove(path))
		}
	}

	if stopServices {
		log.Info("Removing kubelet files in /var/lib/kubelet...")
		_, err := s.Exec("sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'").String()
		cobra.CheckErr(err)
	}

	if resetCni {
		cobra.CheckErr(deleteCniNamespaces())
		cobra.CheckErr(deleteNetworkInterfaces())
	}

	if resetIptables {
		log.Info("Cleaning up iptables rules...")
		_, err := s.Exec("iptables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("iptables-restore").String()
		cobra.CheckErr(err)
		log.Info("Cleaning up ip6tables rules...")
		_, err = s.Exec("ip6tables-save").Reject("KUBE-").Reject("CNI-").Reject("FLANNEL").Exec("ip6tables-restore").String()
		cobra.CheckErr(err)
	}

	if resetIpAddress && ikniteConfig.CreateIp {
		log.Info("Resetting IP address...")
		hosts, err := txeh.NewHosts(&txeh.HostsConfig{})
		cobra.CheckErr(err)
		ip, err := alpine.IpMappingForHost(hosts, ikniteConfig.DomainName)
		cobra.CheckErr(err)
		ones, _ := ip.DefaultMask().Size()
		ipWithMask := fmt.Sprintf("%v/%d", ip, ones)

		p := s.Exec("ip -br -4 a sh").Match(ipWithMask).Column(1).FilterLine(func(s string) string {
			log.WithField("interface", s).WithField("ip", ipWithMask).Debug("Deleting IP address...")
			return s
		}).ExecForEach(fmt.Sprintf("ip addr del %s dev {{.}}", ipWithMask))
		p.Wait()
		cobra.CheckErr(p.Error())
		hosts.RemoveHost(ikniteConfig.DomainName)
		cobra.CheckErr(hosts.Save())
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

func deleteCniNamespaces() error {
	log.Info("Deleting CNI namespaces...")
	p := s.Exec("ip netns show").Column(1).FilterLine(func(s string) string {
		log.WithField("namespace", s).Debug("Deleting namespace...")
		return s
	}).ExecForEach("ip netns delete {{.}}")
	p.Wait()
	return p.Error()
}

func deleteNetworkInterfaces() error {
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
