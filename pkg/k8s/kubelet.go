package k8s

// cSpell:words godotenv txeh
// cSpell: disable
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

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
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
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		pidFilePath = fmt.Sprintf("/var/run/supervise-%s.pid", service)
		pidBytes, err = os.ReadFile(pidFilePath)
	}
	if err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		var pid int
		pid, err = strconv.Atoi(pidStr)
		if err != nil {
			logger.WithField("content", pidStr).Warn("Failed to convert pid file to integer")
		} else {
			var process *os.Process
			process, err = os.FindProcess(pid)
			if err == nil && process.Signal(syscall.Signal(0)) == nil {
				if cmd != nil {
					cmd.Process = process
				}
				return pid, nil
			} else {
				logger.WithField("pid", pid).Warn("Pidfile contained an invalid pid")
				// remove kubeletPidFile
				err = os.Remove(pidFilePath)
				if err != nil {
					return 0, fmt.Errorf("failed to remove pid file: %w", err)
				}
			}
		}
	} else {
		// only return error is the error is not a file not found error
		if !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("failed to read pid file: %w", err)
		}
	}
	return 0, nil
}

// IsKubeletRunning checks if the kubelet process is running.
func IsKubeletRunning() (*os.Process, error) {
	pidBytes, err := os.ReadFile(kubeletPidFile)
	if err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		var pid int
		pid, err = strconv.Atoi(pidStr)
		if err != nil {
			log.WithField("pidfile", kubeletPidFile).
				Warnf("Failed to convert kubelet PID to integer: %s", pidStr)
		} else {
			var process *os.Process
			process, err = os.FindProcess(pid)
			if err == nil && process.Signal(syscall.Signal(0)) == nil {
				log.WithField("pid", pid).Warnf("Kubelet is already running with pid: %d. Swallowing...", pid)
				return process, nil
			} else {
				log.WithField("pid", pid).Warnf("Kubelet pidfile contained an invalid pid: %d", pid)
				// remove kubeletPidFile
				err = os.Remove(kubeletPidFile)
				if err != nil {
					return nil, fmt.Errorf("failed to remove kubelet pidfile: %w", err)
				}
			}
		}
	} else {
		// only return error is the error is not a file not found error
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read kubelet pid file: %w", err)
		}
	}
	return nil, nil
}

func StartKubelet() (*exec.Cmd, error) {
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

	// Check if a process with the value contained in kubeletPidFile exists
	// ignore the error if for some reason the pid file is not found
	kubeletPid, _ := CheckPidFile("kubelet", cmd)
	if kubeletPid > 0 {
		log.WithField("pid", kubeletPid).
			Warnf("Kubelet is already running with pid: %d. Swallowing...", kubeletPid)
		return cmd, nil
	}

	// Create the kubelet log directory if it doesn't exist
	err = os.MkdirAll(kubeletLogDir, os.ModePerm)
	if err != nil {
		return nil, errors.WithMessagef(
			err,
			"Failed to create kubelet log directory %s",
			kubeletLogDir,
		)
	}

	// Open the kubelet log file for writing
	logFile, err := os.OpenFile(kubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to open kubelet log file %s", kubeletLogFile)
	}
	defer func() {
		_ = logFile.Close()
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
		return nil, errors.WithMessage(err, "Failed to start subprocess")
	}

	// Write the PID to the /run/kubelet.pid file
	err = os.WriteFile(kubeletPidFile, fmt.Appendf(nil, "%d", cmd.Process.Pid), 0o644)
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
	_ = os.Remove(kubeletPidFile)
}

func StartAndConfigureKubelet(kubeConfig *v1alpha1.IkniteClusterSpec) error {
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
		_ = cmd.Wait()
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
				killKubelet()
			} else {
				log.Info("API server is healthy")
				go func() {
					force_config := viper.GetBool(config.ForceConfig)
					config, err := LoadFromDefault()
					if err != nil {
						configErr <- err
					} else {
						configErr <- config.DoKustomization(kubeConfig.Ip, kubeConfig.Kustomization, force_config, 0)
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

func CheckKubeletRunning(retries, okResponses, waitTime int) (err error) {
	okTries := 0
	for retries > 0 {
		var resp *http.Response
		log.WithField("retries", retries).Debug("Checking kubelet health...")
		resp, err = http.Get("http://localhost:10248/healthz")
		if err == nil {
			defer func() { err = resp.Body.Close() }()
			var body []byte
			body, err = io.ReadAll(resp.Body)
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
				"wait_time": waitTime,
			}).Debug("Waiting...")
			time.Sleep(time.Duration(waitTime) * time.Millisecond)
		}
	}
	return err
}
