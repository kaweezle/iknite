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

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	kubeletEnvFile        = "/etc/conf.d/kubelet"
	kubeAdmFlagsFile      = "/var/lib/kubelet/kubeadm-flags.env"
	kubeletPidFile        = "/run/kubelet.pid"
	kubeletLogDir         = "/var/log/kubelet"
	kubeletLogFile        = "/var/log/kubelet/kubelet.log"
	iknitePidFile         = "/run/iknite.pid"
	kubeletArgsEnv        = "command_args"
	kubeletKubeadmArgsEnv = "KUBELET_KUBEADM_ARGS"
)

func StartKubelet(kubeConfig *IkniteConfig) error {

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
				return fmt.Errorf("kubelet is already running with pid: %d", pid)
			}
		}
	}

	// Read the environment variables from /var/lib/kubelet/kubeadm-flags.env
	log.WithField("kubeletEnvFile", kubeletEnvFile).Info("Reading kubelet environment file")

	envData, err := godotenv.Read(kubeletEnvFile, kubeAdmFlagsFile)
	if err != nil {
		return errors.WithMessagef(err, "Failed to read environment file %s", kubeletEnvFile)
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
		return errors.WithMessagef(err, "Failed to create kubelet log directory %s", kubeletLogDir)
	}

	// Open the kubelet log file for writing
	logFile, err := os.OpenFile(kubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return errors.WithMessagef(err, "Failed to open kubelet log file %s", kubeletLogFile)
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
		return errors.WithMessage(err, "Failed to start subprocess")
	}

	// Write the PID to the /run/kubelet.pid file
	err = os.WriteFile(kubeletPidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)
	if err != nil {
		return errors.WithMessage(err, "Failed to write PID file")
	}

	// Write the current PID to the /run/iknite.pid file
	err = os.WriteFile(iknitePidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		return errors.WithMessage(err, "Failed to write PID file")
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

	defer func() {
		os.Remove(kubeletPidFile)
		os.Remove(iknitePidFile)
	}()

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
