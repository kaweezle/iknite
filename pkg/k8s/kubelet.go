package k8s

// cSpell:words godotenv txeh
// cSpell: disable
import (
	"context"
	"errors"
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

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
)

// cSpell: enable

const (
	kubeletEnvFile        = "/etc/conf.d/kubelet"
	kubeAdmFlagsFile      = "/var/lib/kubelet/kubeadm-flags.env"
	kubeletPidFile        = "/run/kubelet.pid"
	kubeletLogDir         = "/var/log/kubelet"
	kubeletLogFile        = "/var/log/kubelet/kubelet.log"
	kubeletArgsEnv        = "command_args"
	kubeletKubeadmArgsEnv = "KUBELET_KUBEADM_ARGS"
)

// cSpell: disable
var (
	pathsToUnmount = []string{
		"/var/lib/kubelet/pods",
		"/var/lib/kubelet/plugins",
		"/var/lib/kubelet",
	}
	pathsToUnmountAndRemove = []string{"/run/containerd", "/run/netns", "/run/ipcns", "/run/utsns"}
)

// cSpell: enable

func CheckPidFile(service string, cmd *exec.Cmd) (int, error) {
	pidFilePath := fmt.Sprintf("/run/%s.pid", service)
	logger := log.WithField("pidfile", pidFilePath)
	pidBytes, err := os.ReadFile(pidFilePath) //nolint:gosec // Controlled file path
	if err != nil && errors.Is(err, os.ErrNotExist) {
		pidFilePath = fmt.Sprintf("/var/run/supervise-%s.pid", service)
		pidBytes, err = os.ReadFile(pidFilePath) //nolint:gosec // Controlled file path
	}
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// only return error is the error is not a file not found error
			return 0, fmt.Errorf("failed to read pid file: %w", err)
		}
		return 0, nil
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	var pid int
	pid, err = strconv.Atoi(pidStr)
	if err != nil {
		logger.WithField("content", pidStr).Warn("Failed to convert pid file to integer")
		return 0, fmt.Errorf("Failed to convert pid file to integer: %w", err)
	}
	var process *os.Process
	process, err = os.FindProcess(pid)
	if err == nil && process.Signal(syscall.Signal(0)) == nil {
		if cmd != nil {
			cmd.Process = process
		}
		return pid, nil
	}
	logger.WithField("pid", pid).Warn("Pidfile contained an invalid pid")
	// remove kubeletPidFile
	err = os.Remove(pidFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to remove pid file: %w", err)
	}
	return 0, nil
}

// IsKubeletRunning checks if the kubelet process is running.
func IsKubeletRunning() (*os.Process, error) {
	pidBytes, err := os.ReadFile(kubeletPidFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// only return error is the error is not a file not found error
			return nil, fmt.Errorf("failed to read kubelet pid file: %w", err)
		}
		return nil, nil //nolint:nilnil // means not running
	}
	pidStr := strings.TrimSpace(string(pidBytes))
	var pid int
	pid, err = strconv.Atoi(pidStr)
	if err != nil {
		log.WithField("pidfile", kubeletPidFile).
			Warnf("Failed to convert kubelet PID to integer: %s", pidStr)
		return nil, fmt.Errorf("Failed to convert kubelet PID to integer: %s: %w", pidStr, err)
	}
	var process *os.Process
	process, err = os.FindProcess(pid)
	if err == nil && process.Signal(syscall.Signal(0)) == nil {
		log.WithField("pid", pid).
			Warnf("Kubelet is already running with pid: %d. Swallowing...", pid)
		return process, nil
	}
	log.WithField("pid", pid).Warnf("Kubelet pidfile contained an invalid pid: %d", pid)
	// remove kubeletPidFile
	err = os.Remove(kubeletPidFile)
	if err != nil {
		return nil, fmt.Errorf("failed to remove kubelet pidfile: %w", err)
	}
	return nil, nil //nolint:nilnil // means not running (old pid file)
}

func StartKubelet() (*exec.Cmd, error) {
	// Read the environment variables from /var/lib/kubelet/kubeadm-flags.env
	log.WithField("kubeletEnvFile", kubeletEnvFile).Info("Reading kubelet environment file")

	envData, err := godotenv.Read(kubeletEnvFile, kubeAdmFlagsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment file %s: %w", kubeletEnvFile, err)
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
	cmd := exec.CommandContext(context.Background(), "/usr/bin/kubelet")
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, env...)

	// Check if a process with the value contained in kubeletPidFile exists
	// ignore the error if for some reason the pid file is not found
	kubeletPid, err := CheckPidFile("kubelet", cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to check kubelet pid file %s: %w", kubeletPidFile, err)
	}
	if kubeletPid > 0 {
		log.WithField("pid", kubeletPid).
			Warnf("Kubelet is already running with pid: %d. Swallowing...", kubeletPid)
		return cmd, nil
	}

	// Create the kubelet log directory if it doesn't exist
	err = os.MkdirAll(kubeletLogDir, 0o755) //nolint:gosec // Want read access
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create kubelet log directory %s: %w",
			kubeletLogDir,
			err,
		)
	}

	// Open the kubelet log file for writing
	logFile, err := os.OpenFile( //nolint:gosec // Want read access
		kubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open kubelet log file %s: %w", kubeletLogFile, err)
	}
	defer func() {
		err = logFile.Close()
	}()

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
		return nil, fmt.Errorf("failed to start subprocess: %w", err)
	}

	// Write the PID to the /run/kubelet.pid file
	err = os.WriteFile( //nolint:gosec // Want read access
		kubeletPidFile, fmt.Appendf(nil, "%d", cmd.Process.Pid), 0o644)
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
	err := os.Remove(kubeletPidFile)
	if err != nil {
		log.WithFields(log.Fields{
			"err":     err,
			"pidFile": kubeletPidFile,
		}).Warn("Failed to remove kubelet PID file")
	}
}

func StartAndConfigureKubelet(kubeConfig *v1alpha1.IkniteClusterSpec) error {
	cmd, err := StartKubelet()
	if err != nil {
		return fmt.Errorf("failed to start kubelet: %w", err)
	}

	// Wait for SIGTERM and SIGKILL signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	kubeletHealthz, apiServerHealthz, configErr := make(
		chan error,
		1,
	), make(
		chan error,
		1,
	), make(
		chan error,
		1,
	)
	go func() {
		kubeletHealthz <- CheckKubeletRunning(10, 3, 1000)
	}()

	defer RemovePidFiles()

	alive := true

	killKubelet := func() {
		err = cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Fatalf("Failed to stop subprocess: %v", err)
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatalf("Failed to stop subprocess: %v", err)
		}
		alive = false
	}

	// Wait for the signals or for the child process to stop
	for alive {
		select {
		case <-stop:
			// Stop the cmd process
			log.Info("Received TERM Signal. Stopping kubelet...")
			killKubelet()
		case <-cmdDone:
			// Child process has stopped
			log.Infof("Kubelet stopped with state: %s", cmd.ProcessState.String())
			alive = false
		case isKubeletHealthy := <-kubeletHealthz:
			if isKubeletHealthy != nil {
				log.WithError(isKubeletHealthy).Error("Kubelet is not healthy")
				killKubelet()
			} else {
				log.Info("Kubelet is healthy. Waiting for API server to be healthy...")
				go func() {
					apiConfig, err := LoadFromDefault()
					if err != nil {
						apiServerHealthz <- err
					} else {
						apiServerHealthz <- apiConfig.CheckClusterRunning(30, 2, 1000)
					}
				}()
			}
		case isApiServerHealthy := <-apiServerHealthz:
			if isApiServerHealthy != nil {
				log.WithError(isApiServerHealthy).Error("API server is not healthy")
				killKubelet()
			} else {
				log.Info("API server is healthy")
				go func() {
					force_config := viper.GetBool(config.ForceConfig)
					apiConfig, err := LoadFromDefault()
					if err != nil {
						configErr <- err
					} else {
						configErr <- apiConfig.DoKustomization(kubeConfig.Ip, kubeConfig.Kustomization, force_config, 0)
					}
				}()
			}
		case configError := <-configErr:
			if configError != nil {
				log.WithError(configError).Error("Failed to configure the cluster")
				killKubelet()
			} else {
				log.Info("Cluster configured successfully")
			}
		}
	}

	return nil
}

func CheckKubeletRunning(retries, okResponses, waitTime int) error {
	okTries := 0
	first := true
	client := http.DefaultClient
	var err error
	for ; retries > 0; retries-- {
		if !first {
			log.WithFields(log.Fields{
				"err":       err,
				"wait_time": waitTime,
			}).Debug("Waiting...")
			time.Sleep(time.Duration(waitTime) * time.Millisecond)
		}
		first = false
		var req *http.Request
		var resp *http.Response
		log.WithField("retries", retries).Debug("Checking kubelet health...")

		req, err = http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"http://localhost:10248/healthz",
			http.NoBody,
		)
		if err != nil {
			log.WithError(err).Debug("while making HTTP request")
			continue
		}
		resp, err = client.Do(req)
		if err != nil {
			log.WithError(err).Debug("while making HTTP request")
			continue
		}

		defer func() { err = resp.Body.Close() }() //nolint:gocritic // TODO: check potential leak
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Debug("while reading response body")
			continue
		}
		contentStr := string(body)
		if contentStr != "ok" {
			err = fmt.Errorf("cluster health API returned: %s", contentStr)
			log.WithError(err).Debug("Bad response")
		} else {
			okTries += 1
			log.WithField("okTries", okTries).Trace("Ok response from server")
			if okTries == okResponses {
				break
			}
		}
	}
	if retries == 0 && okTries < okResponses {
		log.Trace("No more retries left.")
	}
	return err
}
