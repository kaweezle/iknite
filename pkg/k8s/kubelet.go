package k8s

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	s "github.com/bitfield/script"
	"github.com/joho/godotenv"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/txn2/txeh"
)

const (
	kubeletEnvFile        = "/etc/conf.d/kubelet"
	kubeAdmFlagsFile      = "/var/lib/kubelet/kubeadm-flags.env"
	kubeletPidFile        = "/run/kubelet.pid"
	kubeletLogDir         = "/var/log/kubelet"
	kubeletLogFile        = "/var/log/kubelet/kubelet.log"
	kubeletArgsEnv        = "command_args"
	kubeletKubeadmArgsEnv = "KUBELET_KUBEADM_ARGS"
)

var (
	pathsToUnmount          = []string{"/var/lib/kubelet/pods", "/var/lib/kubelet/plugins", "/var/lib/kubelet"}
	pathsToUnmountAndRemove = []string{"/run/containerd", "/run/netns", "/run/ipcns", "/run/utsns"}
)

func StartKubelet() (*exec.Cmd, error) {
	// Check if a process with the value contained in kubeletPidFile exists
	pidBytes, err := os.ReadFile(kubeletPidFile)
	if err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			log.WithField("pidfile", kubeletPidFile).Warnf("Failed to convert kubelet PID to integer: %s", pidStr)
		} else {
			process, err := os.FindProcess(pid)
			if err == nil && process.Signal(syscall.Signal(0)) == nil {
				return nil, fmt.Errorf("kubelet is already running with pid: %d", pid)
			}
		}
	}

	// Read the environment variables from /var/lib/kubelet/kubeadm-flags.env
	log.WithField("kubeletEnvFile", kubeletEnvFile).Info("Reading kubelet environment file")

	envData, err := godotenv.Read(kubeletEnvFile, kubeAdmFlagsFile)
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to read environment file %s", kubeletEnvFile)
	}

	args := make([]string, 0)
	if val, ok := envData[kubeletArgsEnv]; ok {
		args = append(args, strings.Fields(val)...)
	}
	if val, ok := envData[kubeletKubeadmArgsEnv]; ok {
		args = append(args, strings.Fields(val)...)
	}

	env := make([]string, 0)
	for k, v := range envData {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Run the command in a subprocess
	cmd := exec.Command("/usr/bin/kubelet")
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, env...)
	// Create the kubelet log directory if it doesn't exist
	err = os.MkdirAll(kubeletLogDir, os.ModePerm)
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to create kubelet log directory %s", kubeletLogDir)
	}

	// Open the kubelet log file for writing
	logFile, err := os.OpenFile(kubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to open kubelet log file %s", kubeletLogFile)
	}
	defer logFile.Close()

	// Set the command's stdout and stderr to the log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	log.WithFields(log.Fields{
		"args":    cmd.Args,
		"argsLen": len(cmd.Args),
		"env":     cmd.Env,
		"logFile": kubeletLogFile,
		"pidFile": kubeletPidFile,
	}).Info("Starting kubelet...")

	// Start the subprocess and get the PID
	err = cmd.Start()
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to start subprocess")
	}

	// Write the PID to the /run/kubelet.pid file
	err = os.WriteFile(kubeletPidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)
	if err != nil {
		log.WithFields(log.Fields{
			"pid":     cmd.Process.Pid,
			"err":     err,
			"pidFile": kubeletPidFile,
		}).Warn("Failed to write kubelet PID file")
	}

	return cmd, nil
}

func RemovePidFiles() {
	os.Remove(kubeletPidFile)
}

func StartAndConfigureKubelet(kubeConfig *IkniteConfig) error {

	cmd, err := StartKubelet()
	if err != nil {
		return errors.Wrap(err, "Failed to start kubelet")
	}

	// Wait for SIGTERM and SIGKILL signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	kubeletHealthz, apiServerHealthz, configErr := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	go func() {
		kubeletHealthz <- CheckKubeletRunning(10, 3, 1000)
	}()

	defer RemovePidFiles()

	// Wait for the signals or for the child process to stop
	for alive := true; alive; {
		select {
		case <-stop:
			// Stop the cmd process
			log.Info("Recevied TERM Signal. Stopping kubelet...")
			err = cmd.Process.Signal(syscall.SIGTERM)
			if err != nil {
				log.Fatalf("Failed to stop subprocess: %v", err)
			}
			cmd.Wait()
			alive = false
		case <-cmdDone:
			// Child process has stopped
			log.Infof("Kubelet stopped with state: %s", cmd.ProcessState.String())
			alive = false
		case isKubeletHealthy := <-kubeletHealthz:
			if isKubeletHealthy != nil {
				log.WithError(isKubeletHealthy).Error("Kubelet is not healthy")
				err = cmd.Process.Signal(syscall.SIGTERM)
				if err != nil {
					log.Fatalf("Failed to stop subprocess: %v", err)
				}
				cmd.Wait()
				alive = false
			} else {
				log.Info("Kubelet is healthy. Waiting for API server to be healthy...")
				go func() {
					config, err := LoadFromDefault()
					if err != nil {
						apiServerHealthz <- err
					} else {
						apiServerHealthz <- config.CheckClusterRunning(30, 2, 1000)
					}
				}()
			}
		case isApiServerHealthy := <-apiServerHealthz:
			if isApiServerHealthy != nil {
				log.WithError(isApiServerHealthy).Error("API server is not healthy")
				err = cmd.Process.Signal(syscall.SIGTERM)
				if err != nil {
					log.Fatalf("Failed to stop subprocess: %v", err)
				}
				cmd.Wait()
				alive = false
			} else {
				log.Info("API server is healthy")
				go func() {
					force_config := viper.GetBool("force_config")
					config, err := LoadFromDefault()
					if err != nil {
						configErr <- err
					} else {
						configErr <- config.DoConfiguration(kubeConfig.Ip, force_config, 0)
					}
				}()
			}
		case configError := <-configErr:
			if configError != nil {
				log.WithError(configError).Error("Failed to configure the cluster")
				err = cmd.Process.Signal(syscall.SIGTERM)
				if err != nil {
					log.Fatalf("Failed to stop subprocess: %v", err)
				}
				cmd.Wait()
				alive = false
			} else {
				log.Info("Cluster configured successfully")
			}
		}
	}

	return nil
}

func CheckKubeletRunning(retries, okResponses, waitTime int) (err error) {
	okTries := 0
	for retries > 0 {
		var resp *http.Response
		log.WithField("retries", retries).Debug("Checking kubelet health...")
		resp, err = http.Get("http://localhost:10248/healthz")
		if err == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				contentStr := string(body)
				if contentStr != "ok" {
					err = fmt.Errorf("cluster health API returned: %s", contentStr)
					log.WithError(err).Debug("Bad response")
				} else {
					okTries = okTries + 1
					log.WithField("okTries", okTries).Trace("Ok response from server")
					if okTries == okResponses {
						break
					}
				}
			} else {
				log.WithError(err).Debug("while reading response body")
			}
		} else {
			log.WithError(err).Debug("while making HTTP request")
		}
		retries = retries - 1
		if retries == 0 {
			log.Trace("No more retries left.")
			return err
		} else {
			log.WithFields(log.Fields{
				"err":       err,
				"wait_time": waitTime}).Debug("Waiting...")
			time.Sleep(time.Duration(waitTime) * time.Millisecond)
		}
	}
	return err
}

func ResetIPAddress(ikniteConfig *IkniteConfig) error {
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

func CleanAll(ikniteConfig *IkniteConfig) {

	var err error
	log.Info("Stopping all containers...")
	if _, err = s.Exec("/bin/zsh -c 'export CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock;crictl rmp -f $(crictl pods -q)'").String(); err != nil {
		log.WithError(err).Warn("Error stopping all containers")
	}

	for _, path := range pathsToUnmount {
		err = doUnmount(path)
		if err != nil {
			log.WithError(err).Warn("Error unmounting path")
		}
	}

	for _, path := range pathsToUnmountAndRemove {
		err = doUnmountAndRemove(path)
		if err != nil {
			log.WithError(err).Warn("Error unmounting and removing path")
		}
	}

	log.Info("Removing kubelet files in /var/lib/kubelet...")
	_, err = s.Exec("sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'").String()
	if err != nil {
		log.WithError(err).Warn("Error removing kubelet files")
	}

	err = deleteCniNamespaces()
	if err != nil {
		log.WithError(err).Warn("Error deleting CNI namespaces")
	}

	err = deleteNetworkInterfaces()
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

	ResetIPAddress(ikniteConfig)
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
