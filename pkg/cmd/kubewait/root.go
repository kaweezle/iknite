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
// using kstatus (one goroutine per namespace), then optionally clones and runs
// a bootstrap repository script.
package kubewait

// cSpell: words godotenv clientcmd apimachinery kstatus errorf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	defaultBootstrapScript = "iknite-bootstrap.sh"
	defaultBootstrapDir    = "/workspace/bootstrap-repo"
	defaultTimeout         = 10 * time.Minute
	defaultInterval        = 10 * time.Second
	defaultGracePeriod     = 5 * time.Second
)

// Options holds the configuration for the kubewait command.
type Options struct {
	Kubeconfig      string
	BootstrapDir    string
	BootstrapScript string
	RepoURL         string
	RepoRef         string
	EnvFile         string
	Timeout         time.Duration
	Interval        time.Duration
	GracePeriod     time.Duration
	Verbosity       string
	JSONLogs        bool
}

// CreateKubewaitCmd creates the root cobra command for kubewait.
func CreateKubewaitCmd(out io.Writer) *cobra.Command {
	opts := &Options{
		BootstrapScript: defaultBootstrapScript,
		BootstrapDir:    defaultBootstrapDir,
		Timeout:         defaultTimeout,
		Interval:        defaultInterval,
		GracePeriod:     defaultGracePeriod,
		Verbosity:       "info",
	}

	cmd := &cobra.Command{
		Use:   "kubewait [namespaces...]",
		Short: "Wait for Kubernetes workloads to be ready",
		Long: `kubewait waits for all deployments, statefulsets and daemonsets in the
specified namespaces to reach a ready state according to kstatus.

Each namespace is watched concurrently. If a namespace is not yet present at
invocation time the goroutine waits for its creation, then applies a short
grace period to let workloads appear before polling their readiness.

If no namespaces are given, all namespaces present in the cluster at invocation
time are watched.

After all workloads are ready, an optional bootstrap repository is cloned
(when --bootstrap-repo-url and --bootstrap-repo-ref are provided) and then the
bootstrap script inside that directory is executed.

Examples:
  # Wait for workloads in all namespaces
  kubewait

  # Wait for specific namespaces
  kubewait kube-system default

  # Clone and run a bootstrap script after workloads are ready
  kubewait --bootstrap-repo-url git@github.com:org/repo.git --bootstrap-repo-ref main

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
	flags.StringVar(&opts.BootstrapDir, "bootstrap-dir", defaultBootstrapDir,
		"Directory to clone the bootstrap repository into (also used as the script working directory)")
	flags.StringVar(&opts.BootstrapScript, "bootstrap-script", defaultBootstrapScript,
		"Name of the bootstrap script to run inside --bootstrap-dir")
	flags.StringVar(&opts.RepoURL, "bootstrap-repo-url", "",
		"URL of the bootstrap git repository to clone (requires --bootstrap-repo-ref)")
	flags.StringVar(&opts.RepoRef, "bootstrap-repo-ref", "",
		"Git branch or tag to checkout when cloning the bootstrap repository")
	flags.StringVar(&opts.EnvFile, "env-file", "",
		"Path to an env file to load before running the bootstrap script (default: .env inside --bootstrap-dir)")
	flags.DurationVar(&opts.Timeout, "timeout", defaultTimeout,
		"Maximum time to wait for workloads to become ready (0 means wait forever)")
	flags.DurationVar(&opts.Interval, "interval", defaultInterval,
		"Polling interval between readiness checks")
	flags.DurationVar(&opts.GracePeriod, "grace-period", defaultGracePeriod,
		"Grace period to wait after a namespace appears before checking its workloads")
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

	restConfig, err := client.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to build REST config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// If no namespaces were given, list all that exist right now.
	if len(namespaces) == 0 {
		namespaces, err = listNamespaces(ctx, k8sClient, opts.Interval)
		if err != nil {
			return err
		}
	}

	log.Infof("Watching %d namespace(s) concurrently: %v", len(namespaces), namespaces)

	// Launch one goroutine per namespace; collect errors via a buffered channel.
	errCh := make(chan error, len(namespaces))
	var wg sync.WaitGroup
	for _, ns := range namespaces {
		wg.Go(func() {
			if err := waitNamespaceWorkloads(ctx, client, k8sClient, ns, opts); err != nil {
				errCh <- err
			}
		})
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for e := range errCh {
		errs = append(errs, e)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.Info("All workloads in all namespaces are ready")

	// Run the optional bootstrap when a repo URL is provided.
	if opts.RepoURL != "" {
		return runBootstrap(ctx, opts)
	}
	return nil
}

// listNamespaces polls the API server until it can list all namespaces.
func listNamespaces(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	interval time.Duration,
) ([]string, error) {
	log.Info("No namespaces specified, listing all namespaces from the cluster...")

	var names []string
	if err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		list, listErr := k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if listErr != nil {
			log.WithError(listErr).Debug("API not available yet, retrying...")
			return false, nil
		}
		names = make([]string, 0, len(list.Items))
		for _, ns := range list.Items {
			names = append(names, ns.Name)
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return names, nil
}

// waitNamespaceWorkloads waits for a namespace to exist, applies the grace period,
// then polls until every workload in that namespace is ready.
func waitNamespaceWorkloads(
	ctx context.Context,
	client *k8s.RESTClientGetter,
	k8sClient kubernetes.Interface,
	namespace string,
	opts *Options,
) error {
	// 1. Wait for the namespace to exist.
	log.Infof("[%s] Waiting for namespace to exist...", namespace)
	if err := wait.PollUntilContextCancel(ctx, opts.Interval, true, func(ctx context.Context) (bool, error) {
		_, err := k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				log.Debugf("[%s] Namespace not yet present, waiting...", namespace)
				return false, nil
			}
			return false, fmt.Errorf("error checking namespace: %w", err)
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("namespace %s did not appear: %w", namespace, err)
	}

	// 2. Grace period — let workloads be scheduled before polling.
	log.Infof("[%s] Namespace found; waiting grace period %s for workloads to appear", namespace, opts.GracePeriod)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(opts.GracePeriod):
	}

	// 3. Poll workloads until all are ready.
	log.Infof("[%s] Checking workload readiness...", namespace)
	if err := wait.PollUntilContextCancel(
		ctx, opts.Interval, true,
		namespaceWorkloadCondition(client, namespace),
	); err != nil {
		return fmt.Errorf("workloads in namespace %s did not become ready: %w", namespace, err)
	}

	log.Infof("[%s] All workloads ready", namespace)
	return nil
}

// namespaceWorkloadCondition returns a ConditionWithContextFunc that polls workload readiness
// for a single namespace.
func namespaceWorkloadCondition(
	client *k8s.RESTClientGetter,
	namespace string,
) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		states, err := client.WorkloadStatesForNamespace(namespace)
		if err != nil {
			log.WithError(err).Warnf("[%s] Failed to get workload states, will retry", namespace)
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
			"namespace": namespace,
			"total":     len(states),
			"ready":     len(ready),
			"unready":   len(unready),
		}).Infof("Workload status: %d/%d ready", len(ready), len(states))

		for _, ws := range unready {
			log.Infof("[%s]   Not ready: %s", namespace, ws.LongString())
		}

		return allReady, nil
	}
}

// runBootstrap clones the bootstrap repository (if URL and ref are provided), loads the env
// file (if present), and executes the bootstrap script.
func runBootstrap(ctx context.Context, opts *Options) error {
	// Clone the repository when a ref is also supplied; if no ref is given the clone is skipped.
	if opts.RepoRef != "" {
		if err := cloneBootstrapRepo(ctx, opts); err != nil {
			return err
		}
	}

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

	if err := os.Chmod(scriptPath, 0o755); err != nil { //nolint:gosec // ensure executable, matching bootstrap.sh chmod +x
		return fmt.Errorf("failed to make bootstrap script executable: %w", err)
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

// cloneBootstrapRepo performs a shallow git clone of the bootstrap repository.
func cloneBootstrapRepo(ctx context.Context, opts *Options) error {
	log.Infof("Cloning bootstrap repository %s (ref: %s) to %s", opts.RepoURL, opts.RepoRef, opts.BootstrapDir)

	// Remove any existing target directory so the clone is clean.
	if err := os.RemoveAll(opts.BootstrapDir); err != nil {
		return fmt.Errorf("failed to remove existing bootstrap dir %s: %w", opts.BootstrapDir, err)
	}

	//nolint:gosec // arguments come from CLI flags under user control
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", opts.RepoRef, opts.RepoURL, opts.BootstrapDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone of %s failed: %w", opts.RepoURL, err)
	}

	return nil
}
