package k8s

// cSpell:words godotenv txeh joho sirupsen ipcns utsns apimachinery
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/errors"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/cmd/util"
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
	Kustomize(ctx context.Context, options *utils.KustomizeOptions) error
	RemovePidFile()
}

func StartKubelet(ctx context.Context, h host.FileExecutor) (host.Process, error) {
	logger := util.LoggerFromContext(ctx)
	// Read the environment variables from /var/lib/kubelet/kubeadm-flags.env
	logger.Info("Reading kubelet environment file", "kubeletEnvFile", KubeletEnvFile)

	// Check if a process with the value contained in kubeletPidFile exists
	// ignore the error if for some reason the pid file is not found
	kubeletPid, p, err := alpine.CheckPidFile(h, KubeletName, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to check kubelet pid file: %w", err)
	}
	if kubeletPid > 0 {
		logger.Warn("Kubelet is already running", "pid", kubeletPid)
		return p, nil
	}

	envData, err := host.ReadEnvFiles(h, logger, KubeletEnvFile, KubeAdmFlagsFile)
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

	logger.Info("Starting kubelet...",
		"args", cmd.Args,
		"argsLen", len(cmd.Args),
		"env", cmd.Env,
		"logFile", KubeletLogFile,
		"pidFile", pidFilePath,
	)

	// Start the subprocess and get the PID
	p, err = h.StartCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start subprocess: %w", err)
	}

	// Write the PID to the /run/kubelet.pid file
	err = h.WriteFile(
		pidFilePath, fmt.Appendf(nil, "%d", p.Pid()), 0o644)
	if err != nil {
		logger.Warn("Failed to write kubelet PID file", utils.ErrorKey, err, "pid", p.Pid(), "pidFile", pidFilePath)
	}

	return p, nil
}

func StartAndConfigureKubelet(
	ctx context.Context,
	runtime KubeletRuntime,
	kustomizeOptions *utils.KustomizeOptions,
) error {
	if runtime == nil {
		return fmt.Errorf("kubelet runtime cannot be nil")
	}
	logger := util.LoggerFromContext(ctx)

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
			logger.Info("Context canceled (SIGTERM ?), stopping kubelet...")
			err = host.TerminateProcess(process, &alive)
		case <-cmdDone:
			// Child process has stopped
			logger.Info("Kubelet stopped", "state", process.State().String())
			alive = false
		case isKubeletHealthy := <-kubeletHealthz:
			if isKubeletHealthy != nil {
				logger.Error("Kubelet is not healthy", utils.ErrorKey, isKubeletHealthy)
				err = host.TerminateProcess(process, &alive)
			} else {
				logger.Info("Kubelet is healthy. Waiting for API server to be healthy...")
				go func() {
					apiServerHealthz <- runtime.CheckClusterRunning(ctx, 30, 2, 10*time.Second)
				}()
			}
		case isApiServerHealthy := <-apiServerHealthz:
			if isApiServerHealthy != nil {
				logger.Error("API server is not healthy", utils.ErrorKey, isApiServerHealthy)
				err = host.TerminateProcess(process, &alive)
			} else {
				logger.Info("API server is healthy")
				go func() {
					configErr <- runtime.Kustomize(
						ctx,
						kustomizeOptions,
					)
				}()
			}
		case configError := <-configErr:
			if configError != nil {
				logger.Error("Failed to configure the cluster", utils.ErrorKey, configError)
				err = configError
				terminateError := host.TerminateProcess(process, &alive)
				if terminateError != nil {
					err = errors.NewAggregate([]error{configError, terminateError})
				}
			} else {
				logger.Info("Cluster configured successfully")
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
	logger := util.LoggerFromContext(ctx)
	var err error
	for ; retries > 0; retries-- {
		if !first {
			logger.Debug("Waiting...", utils.ErrorKey, err, "wait_time", waitTime)
			time.Sleep(waitTime)
		}
		first = false
		var req *http.Request
		var resp *http.Response
		logger.Debug("Checking kubelet health...", "retries", retries)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			url,
			http.NoBody,
		)
		if err != nil {
			err = fmt.Errorf("failed to create HTTP request: %w", err)
			logger.Debug("while making HTTP request", utils.ErrorKey, err)
			continue
		}
		resp, err = client.Do(req)
		if err != nil { // nocov - unlikely to fail
			err = fmt.Errorf("failed to make HTTP request: %w", err)
			logger.Debug("while making HTTP request", utils.ErrorKey, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			logger.Debug("Bad response", utils.ErrorKey, err)
			continue
		}

		defer func() { err = resp.Body.Close() }() //nolint:gocritic // TODO: check potential leak
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil { // nocov - unlikely to fail
			logger.Debug("while reading response body", utils.ErrorKey, err)
			continue
		}
		contentStr := string(body)
		if contentStr != expectedResponse {
			err = fmt.Errorf("unexpected response body: %s", contentStr)
			logger.Debug("Bad response", utils.ErrorKey, err)
		} else {
			okTries += 1
			logger.Debug("Ok response from server", "okTries", okTries)
			if okTries == okResponses {
				break
			}
		}
	}
	if retries == 0 && okTries < okResponses {
		logger.Debug("No more retries left.")
	}
	return err
}

func CheckKubeletRunning(ctx context.Context, retries, okResponses int, waitTime time.Duration) error {
	return CheckServerRunning(ctx, "http://localhost:10248/healthz", "ok", retries, okResponses, waitTime)
}
