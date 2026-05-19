package k8s

// cSpell:words txeh netnsid ifname
// cSpell: disable
import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/txn2/txeh"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s/util"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

type Cleaner struct {
	*slog.Logger
	host.Host
	ikniteConfig *v1alpha1.IkniteClusterSpec
	isDryRun     bool
}

func NewCleaner(
	h host.Host,
	logger *slog.Logger,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	isDryRun bool,
) *Cleaner {
	return &Cleaner{
		Logger:       logger.With("isDryRun", isDryRun),
		Host:         h,
		ikniteConfig: ikniteConfig,
		isDryRun:     isDryRun,
	}
}

func (c *Cleaner) ResetIPAddress() error {
	if !c.ikniteConfig.CreateIp {
		return nil
	}

	c.Info("Resetting IP address...")
	hosts, err := txeh.NewHosts(c.GetHostsConfig())
	if err != nil {
		return fmt.Errorf("failed to create hosts file handler: %w", err)
	}
	ip, err := alpine.IpMappingForHost(hosts, c.ikniteConfig.DomainName)
	if err != nil {
		c.Warn("failed to get IP mapping for host:", utils.ErrorKey, err, "hostname", c.ikniteConfig.DomainName)
		return nil
	}
	ones, _ := ip.DefaultMask().Size()
	ipWithMask := fmt.Sprintf("%v/%d", ip, ones)

	prefix := ""
	if c.isDryRun {
		prefix = "echo "
	}

	p := c.ExecPipe(nil, "ip -br -4 a sh").Match(ipWithMask).Column(1).FilterLine(func(s string) string {
		c.Debug("Deleting IP address...", "interface", s, "ip", ipWithMask)
		return s
	})
	p = c.ExecForEach(p, fmt.Sprintf("%sip addr del %s dev {{.}}", prefix, ipWithMask))
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	if p.Error() != nil {
		return fmt.Errorf("failed to delete IP address: %w", p.Error())
	}
	if !c.isDryRun {
		hosts.RemoveHost(c.ikniteConfig.DomainName)
		if err := hosts.Save(); err != nil {
			return fmt.Errorf("failed to save hosts file: %w", err)
		}
	}
	return nil
}

func (c *Cleaner) ResetIPTables() error {
	c.Info("Cleaning up iptables rules...")
	if !c.isDryRun {
		p := c.ExecPipe(nil, "iptables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL")
		_, err := c.ExecPipe(p, "iptables-restore").String()
		if err != nil {
			return fmt.Errorf("failed to clean up iptables rules: %w", err)
		}
	}
	c.Info("Cleaning up ip6tables rules...")
	if !c.isDryRun {
		p := c.ExecPipe(nil, "ip6tables-save").
			Reject("KUBE-").
			Reject("CNI-").
			Reject("FLANNEL")
		_, err := c.ExecPipe(p, "ip6tables-restore").String()
		if err != nil {
			return fmt.Errorf("failed to clean up ip6tables rules: %w", err)
		}
	}
	return nil
}

//
//nolint:lll // long command
const kubeletDataRemoveCommand = `sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'`

func (c *Cleaner) RemoveKubeletFiles() error {
	if c.isDryRun {
		c.Info(
			"Would remove cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...",
		)
	} else {
		c.Info(
			"Removing cpu_manager_state, memory_manager_state, pod/* files in /var/lib/kubelet...",
		)
		out, err := c.ExecPipe(nil, kubeletDataRemoveCommand).String()
		if err != nil {
			return fmt.Errorf("failed to remove kubelet files: %s: %w", out, err)
		}
	}
	return nil
}

func (c *Cleaner) StopAllContainers() error {
	if c.isDryRun {
		c.Info(
			"Would stop all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...",
		)
	} else {
		c.Info(
			"Stopping all containers with command /bin/sh -c 'crictl rmp -f $(crictl pods -q)'...",
		)
		_, err := c.ExecPipe(nil, "/bin/sh -c 'crictl rmp -f $(crictl pods -q)'").String()
		if err != nil {
			return fmt.Errorf("failed to stop all containers: %w", err)
		}
	}
	return nil
}

func (c *Cleaner) UnmountPaths(failOnError bool) error {
	var err error
	for _, path := range pathsToUnmount {
		err = processMounts(c.Host, path, false, "Unmounting", c.isDryRun, c.Logger)
		if err != nil {
			c.Warn("Error unmounting path", utils.ErrorKey, err, "path", path)
			if failOnError {
				return err
			}
		}
	}

	for _, path := range pathsToUnmountAndRemove {
		err = processMounts(c.Host, path, true, "Unmounting and removing", c.isDryRun, c.Logger)
		if err != nil {
			c.Warn("Error unmounting and removing path", utils.ErrorKey, err, "path", path)
			if failOnError {
				return err
			}
		}
	}
	return nil
}

//nolint:gocyclo // TODO: Should use a runner pattern to reduce complexity
func (c *Cleaner) CleanAll(
	resetIpAddress, resetIpTables, failOnError bool,
) error {
	var err error
	if err = c.StopAllContainers(); err != nil {
		c.Warn("Error stopping all containers", utils.ErrorKey, err)
		if failOnError {
			return err
		}
	}

	err = c.UnmountPaths(failOnError)
	if err != nil {
		c.Warn("Error unmounting paths", utils.ErrorKey, err)
		if failOnError {
			return err
		}
	}

	err = c.RemoveKubeletFiles()
	if err != nil {
		c.Warn("Error removing kubelet files", utils.ErrorKey, err)
		if failOnError {
			return err
		}
	}

	err = c.DeleteCniNamespaces()
	if err != nil {
		c.Warn("Error deleting CNI namespaces", utils.ErrorKey, err)
		if failOnError {
			return err
		}
	}

	err = c.DeleteNetworkInterfaces()
	if err != nil {
		c.Warn("Error deleting network interfaces", utils.ErrorKey, err)
		if failOnError {
			return err
		}
	}

	if resetIpTables {
		c.Info("Cleaning up iptables rules...")
		err = c.ResetIPTables()
		if err != nil {
			c.Warn("Error cleaning up iptables rules", utils.ErrorKey, err)
			if failOnError {
				return err
			}
		}
	}

	if resetIpAddress {
		err = c.ResetIPAddress()
		if err != nil {
			c.Warn("Error resetting IP address", utils.ErrorKey, err)
			if failOnError {
				return err
			}
		}
	}
	return nil
}

func processMounts(
	c host.Host,
	path string,
	remove bool,
	message string,
	isDryRun bool,
	logger *slog.Logger,
) error {
	logger.Info(message, "path", path, "remove", remove)
	var err error
	path, err = c.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to evaluate symlinks for path %s: %w", path, err)
	}

	p := c.Pipe("/proc/self/mounts").Column(2).Match(path).FilterLine(func(s string) string {
		logger.Debug(message, "mount", s)
		if !isDryRun {
			err = c.Unmount(s)
			if err != nil {
				logger.Warn("Error unmounting path", utils.ErrorKey, err, "mount", s)
				return s
			}
			if remove {
				err = c.RemoveAll(s)
				if err != nil {
					logger.Warn("Error removing path", utils.ErrorKey, err, "path", s)
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

func (c *Cleaner) DeleteCniNamespaces() error {
	c.Info("Deleting CNI namespaces...")
	command := "ip netns delete {{.}}"
	if c.isDryRun {
		command = "echo " + command
	}
	p := c.ExecPipe(nil, "ip netns show").Column(1).FilterLine(func(s string) string {
		c.Debug("Deleting namespace...", "namespace", s)
		return s
	})
	p = c.ExecForEach(p, command)
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	if err := p.Error(); err != nil {
		return fmt.Errorf("failed to delete CNI namespaces: %w", err)
	}
	return nil
}

func (c *Cleaner) DeleteNetworkInterfaces() error {
	prefix := ""
	if c.isDryRun {
		prefix = "echo "
	}
	c.Info("Deleting pods network interfaces...")
	//nolint:lll // long string (jq pipeline)
	p := c.ExecPipe(nil, "ip -j link show").
		JQ(`sort_by(.ifname)| reverse | .[] | select((has("link_netnsid") and .ifname != "eth0") or .ifname == "cni0" or .ifname == "flannel.1" or (.ifname | startswith("vip-"))) | .ifname`).
		FilterLine(func(s string) string {
			ifname := s[1 : len(s)-1]
			command := fmt.Sprintf("%sip link delete %s", prefix, ifname)
			c.Debug("Deleting interface with", "interface", ifname, "command", command)
			return command
		})
	p = c.ExecForEach(p, "{{ . }}")
	_ = p.Wait() //nolint:errcheck // we check p.Error() instead
	err := p.Error()
	if err != nil {
		c.Error("Error deleting pods network interfaces", utils.ErrorKey, err)
		return fmt.Errorf("failed to delete network interfaces: %w", err)
	} else {
		c.Info("Deleted pods network interfaces")
	}

	return nil
}

// DeleteAPIBackendData deletes the API backend data directory.
func (c *Cleaner) DeleteAPIBackendData(apiBackendName, apiBackendDatabaseDirectory string) error {
	apiBackendManifestPath := filepath.Join(
		kubeadmConstants.KubernetesDir,
		kubeadmConstants.ManifestsSubDirName,
		apiBackendName+".yaml",
	)
	pod, err := util.ReadStaticPodFromDisk(c, apiBackendManifestPath)
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
	if c.isDryRun {
		c.Info("Dry run: would delete API backend data...", "path", apiBackendDatabaseDirectory)
	} else {
		c.Info("Deleting API backend data...", "path", apiBackendDatabaseDirectory)
		err = host.CleanDir(c, apiBackendDatabaseDirectory)
		if err != nil {
			return fmt.Errorf("failed to delete API backend data: %w", err)
		}
	}
	return nil
}
