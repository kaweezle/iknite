/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kubewait implements the kubewait command.
// It waits for Kubernetes workloads in specified namespaces to become ready
// using kstatus, then optionally runs a bootstrap script.
package kubewait

// cSpell: words godotenv clientcmd apimachinery kstatus

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	defaultBootstrapScript = "iknite-bootstrap.sh"
	defaultTimeout         = 10 * time.Minute
	defaultInterval        = 10 * time.Second
)

// Options holds the configuration for the kubewait command.
type Options struct {
	Kubeconfig      string
	BootstrapDir    string
	BootstrapScript string
	EnvFile         string
	Timeout         time.Duration
	Interval        time.Duration
	Verbosity       string
	JSONLogs        bool
}

// CreateKubewaitCmd creates the root cobra command for kubewait.
func CreateKubewaitCmd(out io.Writer) *cobra.Command {
	opts := &Options{
		BootstrapScript: defaultBootstrapScript,
		Timeout:         defaultTimeout,
		Interval:        defaultInterval,
		Verbosity:       "info",
	}

	cmd := &cobra.Command{
		Use:   "kubewait [namespaces...]",
		Short: "Wait for Kubernetes workloads to be ready",
		Long: `kubewait waits for all deployments, statefulsets and daemonsets in the
specified namespaces to reach a ready state according to kstatus.

If no namespaces are given, all namespaces present in the cluster at invocation
time are watched.

After all workloads are ready, an optional bootstrap script located in the
--bootstrap-dir directory is executed. Environment variables are loaded from a
.env file in that directory (or from --env-file) before the script runs.

Examples:
  # Wait for workloads in all namespaces (uses in-cluster or KUBECONFIG config)
  kubewait

  # Wait for workloads only in kube-system and default namespaces
  kubewait kube-system default

  # Wait and run a bootstrap script
  kubewait --bootstrap-dir /workspace/bootstrap-repo

  # Use a specific kubeconfig
  kubewait --kubeconfig ~/.kube/config kube-system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setUpLogs(out, opts.Verbosity, opts.JSONLogs); err != nil {
				return err
			}
			return run(cmd.Context(), opts, args)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Kubeconfig, "kubeconfig", "",
		"Path to kubeconfig file (defaults to KUBECONFIG env var or ~/.kube/config; falls back to in-cluster config)")
	flags.StringVar(&opts.BootstrapDir, "bootstrap-dir", "",
		"Directory containing the bootstrap repository; if provided the bootstrap script is executed after workloads are ready")
	flags.StringVar(&opts.BootstrapScript, "bootstrap-script", defaultBootstrapScript,
		"Name of the bootstrap script to run inside --bootstrap-dir")
	flags.StringVar(&opts.EnvFile, "env-file", "",
		"Path to an env file to load before running the bootstrap script (default: .env inside --bootstrap-dir)")
	flags.DurationVar(&opts.Timeout, "timeout", defaultTimeout,
		"Maximum time to wait for workloads to become ready (0 means wait forever)")
	flags.DurationVar(&opts.Interval, "interval", defaultInterval,
		"Polling interval between readiness checks")
	flags.StringVarP(&opts.Verbosity, "verbosity", "v", "info",
		"Log level (debug, info, warn, error, fatal, panic)")
	flags.BoolVar(&opts.JSONLogs, "json", false, "Emit log messages as JSON")

	return cmd
}

// Execute is the entry point called from main.
func Execute() {
	cobra.CheckErr(CreateKubewaitCmd(os.Stdout).Execute())
}

// setUpLogs configures logrus output and level.
func setUpLogs(out io.Writer, level string, jsonFormat bool) error {
	log.SetOutput(out)
	if jsonFormat {
		log.SetFormatter(&log.JSONFormatter{})
	}
	lvl, err := log.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level %q: %w", level, err)
	}
	log.SetLevel(lvl)
	return nil
}

// run is the main logic for the kubewait command.
func run(ctx context.Context, opts *Options, namespaces []string) error {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	client := k8s.NewRESTClientGetterFromKubeconfig(opts.Kubeconfig)

	// Resolve namespaces: either use the provided ones or list all existing ones.
	namespaces, err := resolveNamespaces(ctx, client, namespaces, opts.Interval)
	if err != nil {
		return err
	}

	if len(namespaces) > 0 {
		log.Infof("Watching %d namespace(s): %v", len(namespaces), namespaces)
	} else {
		log.Info("Watching all namespaces")
	}

	// Poll until all workloads are ready.
	log.Info("Waiting for workloads to be ready...")
	if err := wait.PollUntilContextCancel(
		ctx, opts.Interval, true,
		workloadCondition(client, namespaces),
	); err != nil {
		return fmt.Errorf("workloads did not become ready: %w", err)
	}

	log.Info("All workloads are ready")

	// Run optional bootstrap script.
	if opts.BootstrapDir != "" {
		return runBootstrap(ctx, opts)
	}
	return nil
}

// resolveNamespaces returns the list of namespaces to watch.
// If namespaces is non-empty it is returned as-is.
// Otherwise the function polls the API server until it can list all namespaces.
func resolveNamespaces(
	ctx context.Context,
	client *k8s.RESTClientGetter,
	namespaces []string,
	interval time.Duration,
) ([]string, error) {
	if len(namespaces) > 0 {
		return namespaces, nil
	}

	log.Info("No namespaces specified, listing all namespaces from the cluster...")

	restConfig, err := client.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	var resolved []string
	if err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		list, listErr := k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if listErr != nil {
			log.WithError(listErr).Debug("API not available yet, retrying...")
			return false, nil
		}
		for _, ns := range list.Items {
			resolved = append(resolved, ns.Name)
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return resolved, nil
}

// workloadCondition returns a ConditionWithContextFunc that polls workload readiness.
func workloadCondition(
	client *k8s.RESTClientGetter,
	namespaces []string,
) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		states, err := client.WorkloadStatesForNamespaces(namespaces)
		if err != nil {
			log.WithError(err).Warn("Failed to get workload states, will retry")
			return false, nil
		}

		allReady := true
		var ready, unready []*v1alpha1.WorkloadState
		for _, s := range states {
			if s.Ok {
				ready = append(ready, s)
			} else {
				allReady = false
				unready = append(unready, s)
			}
		}

		log.WithFields(log.Fields{
			"total":   len(states),
			"ready":   len(ready),
			"unready": len(unready),
		}).Info("Workload status")

		for _, ws := range unready {
			log.Infof("  Not ready: %s", ws.LongString())
		}

		return allReady, nil
	}
}

// runBootstrap sources the env file (if present) and executes the bootstrap script.
func runBootstrap(ctx context.Context, opts *Options) error {
	// Determine the env file path.
	envFile := opts.EnvFile
	if envFile == "" {
		envFile = filepath.Join(opts.BootstrapDir, ".env")
	}

	if info, err := os.Stat(envFile); err == nil && !info.IsDir() {
		log.Infof("Loading environment from %s", envFile)
		if loadErr := godotenv.Load(envFile); loadErr != nil {
			return fmt.Errorf("failed to load env file %s: %w", envFile, loadErr)
		}
	}

	// Locate and execute the bootstrap script.
	scriptPath := filepath.Join(opts.BootstrapDir, opts.BootstrapScript)
	if _, err := os.Stat(scriptPath); err != nil {
		log.Infof("Bootstrap script %s not found in %s, skipping", opts.BootstrapScript, opts.BootstrapDir)
		return nil
	}

	log.Infof("Running bootstrap script: %s", scriptPath)
	//nolint:gosec // scriptPath is controlled by the user via --bootstrap-dir / --bootstrap-script flags
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = opts.BootstrapDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bootstrap script %s failed: %w", scriptPath, err)
	}

	return nil
}
