package k8s

// cSpell:words godotenv txeh joho sirupsen ipcns utsns
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

const (
	KubeletName           = "kubelet"
	KubeletEnvFile        = "/etc/conf.d/kubelet"
	KubeAdmFlagsFile      = "/var/lib/kubelet/kubeadm-flags.env"
	KubeletLogDir         = "/var/log/kubelet"
	KubeletLogFile        = "/var/log/kubelet/kubelet.log"
	kubeletArgsEnv        = "command_args"
	kubeletKubeadmArgsEnv = "KUBELET_KUBEADM_ARGS"
)

var (
	pathsToUnmount = []string{
		"/var/lib/kubelet/pods",
		"/var/lib/kubelet/plugins",
		"/var/lib/kubelet",
	}
	pathsToUnmountAndRemove = []string{"/run/containerd", "/run/netns", "/run/ipcns", "/run/utsns"}
)

type KubeletRuntime interface {
	StartKubelet(ctx context.Context) (host.Process, error)
	CheckKubeletRunning(ctx context.Context, retries, okResponses int, waitTime time.Duration) error
	CheckClusterRunning(
		ctx context.Context,
		retries, okResponses int,
		interval time.Duration,
	) error
	Kustomize(ctx context.Context, kustomization string, options *utils.KustomizeOptions) error
	RemovePidFile()
}

func StartKubelet(ctx context.Context, h host.FileExecutor) (host.Process, error) {
	// Read the environment variables from /var/lib/kubelet/kubeadm-flags.env
	log.WithField("kubeletEnvFile", KubeletEnvFile).Info("Reading kubelet environment file")

	// Check if a process with the value contained in kubeletPidFile exists
	// ignore the error if for some reason the pid file is not found
	kubeletPid, p, err := alpine.CheckPidFile(h, "kubelet")
	if err != nil {
		return nil, fmt.Errorf("failed to check kubelet pid file: %w", err)
	}
	if kubeletPid > 0 {
		log.WithField("pid", kubeletPid).
			Warnf("Kubelet is already running with pid: %d. Swallowing...", kubeletPid)
		return p, nil
	}

	envData, err := host.ReadEnvFiles(h, KubeletEnvFile, KubeAdmFlagsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment file %s: %w", KubeletEnvFile, err)
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
	cmd := &host.CommandOptions{
		Cmd:  "/usr/bin/kubelet",
		Args: args,
		Env:  os.Environ(),
	}
	cmd.Env = append(cmd.Env, env...)

	// Create the kubelet log directory if it doesn't exist
	err = h.MkdirAll(KubeletLogDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create kubelet log directory %s: %w",
			KubeletLogDir,
			err,
		)
	}

	// Open the kubelet log file for writing
	logFile, err := h.OpenFile(
		KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open kubelet log file %s: %w", KubeletLogFile, err)
	}
	defer func() {
		err = logFile.Close()
	}()

	// Set the command's stdout and stderr to the log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	pidFilePath := alpine.ServicePidFilePath(KubeletName)

	log.WithFields(log.Fields{
		"args":    cmd.Args,
		"argsLen": len(cmd.Args),
		"env":     cmd.Env,
		"logFile": KubeletLogFile,
		"pidFile": pidFilePath,
	}).Info("Starting kubelet...")

	// Start the subprocess and get the PID
	p, err = h.StartCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start subprocess: %w", err)
	}

	// Write the PID to the /run/kubelet.pid file
	err = h.WriteFile(
		pidFilePath, fmt.Appendf(nil, "%d", p.Pid()), 0o644)
	if err != nil {
		log.WithFields(log.Fields{
			"pid":     p.Pid(),
			"err":     err,
			"pidFile": pidFilePath,
		}).Warn("Failed to write kubelet PID file")
	}

	return p, nil
}

func StartAndConfigureKubelet(
	ctx context.Context,
	runtime KubeletRuntime,
	kubeConfig *v1alpha1.IkniteClusterSpec,
	kustomizeOptions *utils.KustomizeOptions,
) error {
	if runtime == nil {
		return fmt.Errorf("kubelet runtime cannot be nil")
	}

	process, err := runtime.StartKubelet(ctx)
	if err != nil {
		return fmt.Errorf("failed to start kubelet: %w", err)
	}

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- process.Wait()
	}()

	kubeletHealthz, apiServerHealthz, configErr := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	go func() {
		kubeletHealthz <- runtime.CheckKubeletRunning(ctx, 10, 3, 1*time.Second)
	}()

	defer runtime.RemovePidFile()

	// Wait for the signals or for the child process to stop
	for alive := true; alive; {
		select {
		case <-ctx.Done():
			log.Info("Context canceled (SIGTERM ?), stopping kubelet...")
			err = host.TerminateProcess(process, &alive)
		case <-cmdDone:
			// Child process has stopped
			log.Infof("Kubelet stopped with state: %s", process.State().String())
			alive = false
		case isKubeletHealthy := <-kubeletHealthz:
			if isKubeletHealthy != nil {
				log.WithError(isKubeletHealthy).Error("Kubelet is not healthy")
				err = host.TerminateProcess(process, &alive)
			} else {
				log.Info("Kubelet is healthy. Waiting for API server to be healthy...")
				go func() {
					apiServerHealthz <- runtime.CheckClusterRunning(ctx, 30, 2, 10*time.Second)
				}()
			}
		case isApiServerHealthy := <-apiServerHealthz:
			if isApiServerHealthy != nil {
				log.WithError(isApiServerHealthy).Error("API server is not healthy")
				err = host.TerminateProcess(process, &alive)
			} else {
				log.Info("API server is healthy")
				go func() {
					configErr <- runtime.Kustomize(
						ctx,
						kubeConfig.Kustomization,
						kustomizeOptions,
					)
				}()
			}
		case configError := <-configErr:
			if configError != nil {
				log.WithError(configError).Error("Failed to configure the cluster")
				err = host.TerminateProcess(process, &alive)
			} else {
				log.Info("Cluster configured successfully")
			}
		}
	}

	if err != nil {
		return fmt.Errorf("error while waiting for kubelet to stop: %w", err)
	}

	return nil
}

func CheckServerRunning(
	ctx context.Context,
	url, expectedResponse string,
	retries, okResponses int,
	waitTime time.Duration,
) error {
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
			time.Sleep(waitTime)
		}
		first = false
		var req *http.Request
		var resp *http.Response
		log.WithField("retries", retries).Debug("Checking kubelet health...")

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			url,
			http.NoBody,
		)
		if err != nil {
			err = fmt.Errorf("failed to create HTTP request: %w", err)
			log.WithError(err).Debug("while making HTTP request")
			continue
		}
		resp, err = client.Do(req)
		if err != nil { // nocov - unlikely to fail
			err = fmt.Errorf("failed to make HTTP request: %w", err)
			log.WithError(err).Debug("while making HTTP request")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			log.WithError(err).Debug("Bad response")
			continue
		}

		defer func() { err = resp.Body.Close() }() //nolint:gocritic // TODO: check potential leak
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil { // nocov - unlikely to fail
			log.WithError(err).Debug("while reading response body")
			continue
		}
		contentStr := string(body)
		if contentStr != expectedResponse {
			err = fmt.Errorf("unexpected response body: %s", contentStr)
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

func CheckKubeletRunning(ctx context.Context, retries, okResponses int, waitTime time.Duration) error {
	return CheckServerRunning(ctx, "http://localhost:10248/healthz", "ok", retries, okResponses, waitTime)
}
