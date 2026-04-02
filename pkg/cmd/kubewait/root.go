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
// It waits for Kubernetes resources in specified namespaces to become ready
// using kstatus (one goroutine per namespace), then optionally clones and runs
// a bootstrap repository script.
package kubewait

// cSpell: words godotenv clientcmd apimachinery kstatus errorf sirupsen joho metav1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	coreV1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubeUtil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	pollingEvent "sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	defaultBootstrapScript         = "iknite-bootstrap.sh"
	defaultBootstrapDir            = "/workspace"
	defaultEnvFile                 = ".env"
	defaultTimeout                 = 10 * time.Minute
	defaultStatusUpdateInterval    = 4 * time.Second
	defaultResourcesUpdateInterval = 2 * time.Second
	defaultSettlePeriod            = 10 * time.Second
	defaultNamespaceSettlePeriod   = 20 * time.Second
	bootstrapRepoURLEnvVar         = "IKNITE_BOOTSTRAP_REPO_URL"
	bootstrapRepoRefEnvVar         = "IKNITE_BOOTSTRAP_REPO_REF"
	bootstrapScriptEnvVar          = "IKNITE_BOOTSTRAP_SCRIPT"
	bootstrapRepoDirname           = "bootstrap-repo"
)

var defaultResourceTypes = []string{"deployments", "statefulsets", "daemonsets", "jobs", "cronjobs", "applications"}

// Options holds the configuration for the kubewait command.
type Options struct {
	Verbosity               string
	BootstrapDir            string
	BootstrapScript         string
	RepoURL                 string
	RepoRef                 string
	EnvFile                 string
	Kubeconfig              string
	ResourceTypes           []string
	Timeout                 time.Duration
	StatusUpdateInterval    time.Duration
	ResourcesUpdateInterval time.Duration
	SettlePeriod            time.Duration
	NamespaceSettlePeriod   time.Duration
	JSONLogs                bool
	SkipWaitingForResources bool
}

// CreateKubewaitCmd creates the root cobra command for kubewait.
func CreateKubewaitCmd(out io.Writer) *cobra.Command {
	opts := &Options{
		BootstrapScript:         defaultBootstrapScript,
		BootstrapDir:            defaultBootstrapDir,
		Timeout:                 defaultTimeout,
		StatusUpdateInterval:    defaultStatusUpdateInterval,
		ResourcesUpdateInterval: defaultResourcesUpdateInterval,
		SettlePeriod:            defaultSettlePeriod,
		NamespaceSettlePeriod:   defaultNamespaceSettlePeriod,
		Verbosity:               "info",
	}

	cmd := &cobra.Command{
		Use:   "kubewait [namespaces...]",
		Short: "Wait for Kubernetes resources to be ready",
		Long: `kubewait waits for all deployments, statefulsets and daemonsets in the
specified namespaces to reach a ready state according to kstatus.

Each namespace is watched concurrently. If a namespace is not yet present at
invocation time the goroutine waits for its creation, then applies a short
grace period to let resources appear before polling their readiness.

If no namespaces are given, all namespaces present in the cluster at invocation
time are watched.

After all resources are ready, an optional bootstrap repository is cloned
(when --bootstrap-repo-url and --bootstrap-repo-ref are provided) and then the
bootstrap script inside that directory is executed.

Examples:
  # Wait for resources in all namespaces
  kubewait

  # Wait for specific namespaces
  kubewait kube-system default

  # Clone and run a bootstrap script after resources are ready
  kubewait --bootstrap-repo-url git@github.com:org/repo.git --bootstrap-repo-ref main

  # Use a specific kubeconfig
  kubewait --kubeconfig ~/.kube/config kube-system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setUpLogs(out, opts.Verbosity, opts.JSONLogs); err != nil {
				return err
			}
			return runKubewait(cmd.Context(), opts, args)
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
		"Maximum time to wait for resources to become ready (0 means wait forever)")
	flags.DurationVar(&opts.StatusUpdateInterval, "status-update-interval", defaultStatusUpdateInterval,
		"Polling interval between readiness checks")
	flags.DurationVar(&opts.ResourcesUpdateInterval, "resources-update-interval", defaultResourcesUpdateInterval,
		"Polling interval between checks for new or deleted resources in the watched namespaces")
	flags.DurationVar(&opts.SettlePeriod, "settle-period", defaultSettlePeriod,
		"Time to wait after all resources are ready before proceeding to ensure stability")
	flags.DurationVar(&opts.NamespaceSettlePeriod, "namespace-settle-period", defaultNamespaceSettlePeriod,
		"Grace period to wait after a namespace appears before checking its resources")
	flags.StringVarP(&opts.Verbosity, "verbosity", "v", "info",
		"Log level (debug, info, warn, error, fatal, panic)")
	flags.BoolVar(&opts.JSONLogs, "json", false, "Emit log messages as JSON")
	flags.StringSliceVar(
		&opts.ResourceTypes,
		"resource-types",
		defaultResourceTypes,
		"Comma-separated list of resource types to check",
	)
	flags.BoolVar(
		&opts.SkipWaitingForResources,
		"skip-wait",
		false,
		"Skip waiting for resources to be ready and proceed directly to the optional bootstrap (for testing purposes)",
	)

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

// waitForResources waits for all resources in the specified namespaces to become ready.
func waitForResources(ctx context.Context, opts *Options, namespaces []string) error {
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

	// Validate that the requested resource types are supported by the cluster before starting the wait loops.
	validTypes, err := client.ValidateResourceTypes(opts.ResourceTypes)
	if err != nil {
		return fmt.Errorf("resource type validation failed: %w", err)
	}
	if len(validTypes) == 0 {
		return errors.New("none of the specified resource types are supported by the cluster")
	}
	opts.ResourceTypes = validTypes

	// If no namespaces were given, list all that exist right now.
	if len(namespaces) == 0 {
		namespaces, err = listNamespaces(ctx, k8sClient, opts.StatusUpdateInterval)
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
			if err := waitNamespaceResources(ctx, client, k8sClient, ns, opts); err != nil {
				errCh <- err
			}
		})
	}
	wg.Wait()
	close(errCh)

	errs := make([]error, 0, len(namespaces))
	for e := range errCh {
		errs = append(errs, e)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.Info("All resources in all namespaces are ready")
	return nil
}

// runKubewait is the main logic for the kubewait command.
func runKubewait(ctx context.Context, opts *Options, namespaces []string) error {
	if !opts.SkipWaitingForResources {
		if err := waitForResources(ctx, opts, namespaces); err != nil {
			return fmt.Errorf("error while waiting for resources: %w", err)
		}
	}

	if err := runBootstrap(ctx, opts); err != nil {
		return fmt.Errorf("error during bootstrap: %w", err)
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
		for i := range list.Items {
			ns := &list.Items[i]
			names = append(names, ns.Name)
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	return names, nil
}

type resourceWaiter struct {
	logger             log.FieldLogger
	client             *k8s.RESTClientGetter
	poller             *polling.StatusPoller
	pollCancel         context.CancelFunc
	watchDatasetCancel context.CancelFunc
	settleTimer        *time.Timer
	endChannel         chan error
	opts               *Options
	namespace          string
	currentDataSet     object.ObjMetadataSet
	mu                 sync.Mutex
}

func newResourceWaiter(
	client *k8s.RESTClientGetter,
	namespace string,
	opts *Options,
) (*resourceWaiter, error) {
	factory := kubeUtil.NewFactory(client)
	poller, err := polling.NewStatusPollerFromFactory(factory, polling.Options{
		CustomStatusReaders: []engine.StatusReader{&k8s.ApplicationStatusReader{}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create status poller: %w", err)
	}

	logger := log.WithField("namespace", namespace)

	return &resourceWaiter{
		client:         client,
		namespace:      namespace,
		poller:         poller,
		logger:         logger,
		pollCancel:     nil,
		settleTimer:    nil,
		endChannel:     make(chan error, 1),
		currentDataSet: object.ObjMetadataSet{},
		opts:           opts,
	}, nil
}

func (w *resourceWaiter) StartSettleTimer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.settleTimer != nil {
		return nil
	}
	w.settleTimer = time.AfterFunc(w.opts.SettlePeriod, func() {
		w.reportDone(nil)
	})

	return nil
}

func (w *resourceWaiter) StopSettleTimer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.settleTimer == nil {
		return nil
	}
	w.settleTimer.Stop()
	w.settleTimer = nil
	return nil
}

func (w *resourceWaiter) hasSettleTimer() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.settleTimer != nil
}

func (w *resourceWaiter) stopWatchingDataSetChanges() {
	w.mu.Lock()
	cancel := w.watchDatasetCancel
	w.watchDatasetCancel = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (w *resourceWaiter) reportDone(err error) {
	w.stopWatchingDataSetChanges()
	select {
	case w.endChannel <- err:
	default:
	}
}

func (w *resourceWaiter) stopPolling() {
	w.mu.Lock()
	cancel := w.pollCancel
	w.pollCancel = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (w *resourceWaiter) setCurrentDataSet(dataSet object.ObjMetadataSet) {
	w.mu.Lock()
	w.currentDataSet = dataSet
	w.mu.Unlock()
}

func (w *resourceWaiter) getCurrentDataSet() object.ObjMetadataSet {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append(object.ObjMetadataSet(nil), w.currentDataSet...)
}

func (w *resourceWaiter) startPolling(ctx context.Context, dataSet object.ObjMetadataSet) {
	pollCtx, cancel := context.WithCancel(ctx)

	w.mu.Lock()
	w.pollCancel = cancel
	w.mu.Unlock()

	w.logger.WithFields(log.Fields{
		"pollInterval":  w.opts.StatusUpdateInterval.Round(time.Second),
		"resourceCount": len(dataSet),
	}).Info("Waiting for resources to become ready")

	coll := collector.NewResourceStatusCollector(dataSet)

	eventChannel := w.poller.Poll(pollCtx, dataSet, polling.PollOptions{
		PollInterval: w.opts.StatusUpdateInterval,
	})

	done := coll.ListenWithObserver(eventChannel, collector.ObserverFunc(
		func(rsc *collector.ResourceStatusCollector, event pollingEvent.Event) {
			w.processEvent(rsc, event)
		}),
	)

	go func() {
		for result := range done {
			if result.Err != nil {
				w.logger.Errorf("Error while polling resource statuses: %v", result.Err)
				w.reportDone(result.Err)
				return
			}
		}
	}()
}

func (w *resourceWaiter) refreshDataSet() (object.ObjMetadataSet, error) {
	dataSet, err := w.client.ObjectMetadataSetForNamespace(w.namespace, w.opts.ResourceTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata set for namespace %s: %w", w.namespace, err)
	}

	return dataSet, nil
}

func (w *resourceWaiter) restartPolling(ctx context.Context, newDataSet object.ObjMetadataSet) error {
	w.stopPolling()

	if err := w.StopSettleTimer(); err != nil {
		return err
	}

	w.setCurrentDataSet(newDataSet)
	if len(newDataSet) == 0 {
		w.logger.Info("No resources found in namespace, waiting for settle period")
		if err := w.StartSettleTimer(); err != nil {
			return fmt.Errorf("failed to start settle timer: %w", err)
		}
		return nil
	}

	w.startPolling(ctx, newDataSet)

	return nil
}

func (w *resourceWaiter) watchDataSetChanges(ctx context.Context) {
	if w.opts.ResourcesUpdateInterval <= 0 {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.opts.ResourcesUpdateInterval):
			newDataSet, err := w.refreshDataSet()
			if err != nil {
				w.reportDone(err)
				return
			}

			currentDataSet := w.getCurrentDataSet()
			if currentDataSet.Equal(newDataSet) {
				w.logger.Debug("Resource dataset unchanged")
				continue
			}

			// check that context is not already canceled before restarting the poller
			if ctx.Err() != nil {
				w.logger.Warn("Context canceled, skipping poller restart")
				return
			}

			w.logger.WithFields(log.Fields{
				"previousCount": len(currentDataSet),
				"currentCount":  len(newDataSet),
				"interval":      w.opts.ResourcesUpdateInterval.Round(time.Second),
			}).Info("Resource dataset changed, restarting poller")

			if err := w.restartPolling(ctx, newDataSet); err != nil {
				w.reportDone(err)
				return
			}
		}
	}
}

func (w *resourceWaiter) processEvent(rsc *collector.ResourceStatusCollector, event pollingEvent.Event) {
	if event.Type == pollingEvent.ResourceUpdateEvent {
		w.logger.WithFields(log.Fields{
			"group":     event.Resource.Identifier.GroupKind.Group,
			"kind":      event.Resource.Identifier.GroupKind.Kind,
			"name":      event.Resource.Identifier.Name,
			"namespace": event.Resource.Identifier.Namespace,
			"status":    event.Resource.Status,
			"message":   event.Resource.Message,
		}).Infof("Resource update")
	}
	rss := make([]*pollingEvent.ResourceStatus, 0, len(rsc.ResourceStatuses))
	for _, rs := range rsc.ResourceStatuses {
		rss = append(rss, rs)
	}
	aggStatus := aggregator.AggregateStatus(rss, status.CurrentStatus)
	if aggStatus == status.CurrentStatus && !w.hasSettleTimer() {
		w.logger.WithField("timer", w.opts.SettlePeriod.Round(time.Second)).Info(
			"All resources are ready, starting settle timer",
		)
		if err := w.StartSettleTimer(); err != nil {
			w.logger.Errorf("Failed to start settle timer: %v", err)
		}
	} else if aggStatus != status.CurrentStatus && w.hasSettleTimer() {
		w.logger.Infof("A resource is no longer ready, stopping settle timer")
		if err := w.StopSettleTimer(); err != nil {
			w.logger.Errorf("Failed to stop settle timer: %v", err)
		}
	}
}

func (w *resourceWaiter) Start(ctx context.Context) error {
	dataSet, err := w.refreshDataSet()
	if err != nil {
		return err
	}

	if err := w.restartPolling(ctx, dataSet); err != nil {
		return fmt.Errorf("while starting poll on resources: %w", err)
	}
	watchDatasetContext, cancel := context.WithCancel(ctx)
	w.watchDatasetCancel = cancel

	go w.watchDataSetChanges(watchDatasetContext)

	return nil
}

// waitNamespaceResources waits for a namespace to exist, applies the grace period,
// then polls until every resource in that namespace is ready.
func waitNamespaceResources(
	ctx context.Context,
	client *k8s.RESTClientGetter,
	k8sClient kubernetes.Interface,
	namespace string,
	opts *Options,
) error {
	logger := log.WithField("namespace", namespace)
	// 1. Wait for the namespace to exist.
	logger.Infof("Waiting for namespace to exist...")
	var ns *coreV1.Namespace
	if err := wait.PollUntilContextCancel(
		ctx,
		opts.StatusUpdateInterval,
		true,
		func(ctx context.Context) (bool, error) {
			var err error
			ns, err = k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				if k8errors.IsNotFound(err) {
					logger.Debugf("Namespace not yet present, waiting...")
					return false, nil
				}
				return false, fmt.Errorf("error checking namespace: %w", err)
			}
			return true, nil
		},
	); err != nil {
		return fmt.Errorf("namespace %s did not appear: %w", namespace, err)
	}

	nsExistence := time.Since(ns.CreationTimestamp.Time)
	timeToWait := max(0, opts.NamespaceSettlePeriod-nsExistence)
	logger2 := logger.WithFields(
		log.Fields{
			"namespaceAge": nsExistence.Round(time.Second),
			"settlePeriod": opts.NamespaceSettlePeriod.Round(time.Second),
			"timeToWait":   timeToWait.Round(time.Second),
		},
	)
	if timeToWait > 0 {
		logger2.Info("Namespace younger than grace period, waiting")
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for namespace %s: %w", namespace, ctx.Err())
		case <-time.After(timeToWait):
		}
	} else {
		logger2.Info("Namespace older than grace period, skipping wait")
	}

	waiter, err := newResourceWaiter(client, namespace, opts)
	if err != nil {
		return fmt.Errorf("failed to create resource waiter: %w", err)
	}

	cancellableCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = waiter.Start(cancellableCtx)
	if err != nil {
		return fmt.Errorf("failed to start resource waiter: %w", err)
	}

	for {
		select {
		case <-cancellableCtx.Done():
			return fmt.Errorf("context canceled while polling resources in namespace %s: %w", namespace, ctx.Err())
		case err := <-waiter.endChannel:
			if err != nil {
				return fmt.Errorf("error while polling resources in namespace %s: %w", namespace, err)
			}
			// This case is hit when the settle timer completes,
			// which means all resources have been ready for the entire settle period.
			logger.Info("Namespace ready")
			return nil
		}
	}
}

// runBootstrap clones the bootstrap repository (if URL and ref are provided), loads the env
// file (if present), and executes the bootstrap script.
func runBootstrap(ctx context.Context, opts *Options) error {
	// Clone the repository when a ref is also supplied; if no ref is given the clone is skipped.

	// Determine the env file path.
	envFile := opts.EnvFile
	if envFile == "" {
		envFile = filepath.Join(opts.BootstrapDir, defaultEnvFile)
	}

	if info, err := os.Stat(envFile); err == nil && !info.IsDir() {
		log.Infof("Loading environment from %s", envFile)
		if loadErr := godotenv.Load(envFile); loadErr != nil {
			return fmt.Errorf("failed to load env file %s: %w", envFile, loadErr)
		}
	}

	bootstrapRepoURL, ok := os.LookupEnv(bootstrapRepoURLEnvVar)
	if ok {
		log.Infof("Overriding bootstrap repo URL from env var %s: %s", bootstrapRepoURLEnvVar, bootstrapRepoURL)
		opts.RepoURL = bootstrapRepoURL
	}
	bootstrapRepoRef, ok := os.LookupEnv(bootstrapRepoRefEnvVar)
	if ok {
		log.Infof("Overriding bootstrap repo ref from env var %s: %s", bootstrapRepoRefEnvVar, bootstrapRepoRef)
		opts.RepoRef = bootstrapRepoRef
	}
	bootstrapScript, ok := os.LookupEnv(bootstrapScriptEnvVar)
	if ok {
		log.Infof("Overriding bootstrap script from env var %s: %s", bootstrapScriptEnvVar, bootstrapScript)
		opts.BootstrapScript = bootstrapScript
	}

	baseDir := opts.BootstrapDir
	if opts.RepoURL != "" && opts.RepoRef != "" {
		if err := cloneBootstrapRepo(ctx, opts); err != nil {
			return fmt.Errorf("error during bootstrap: %w", err)
		}
		baseDir = filepath.Join(opts.BootstrapDir, bootstrapRepoDirname)
	} else {
		log.Info("Bootstrap repo URL or ref not provided, skipping clone")
	}

	// Locate and execute the bootstrap script.
	scriptPath := filepath.Join(baseDir, opts.BootstrapScript)
	if _, err := os.Stat(scriptPath); err != nil {
		log.Infof(
			"Bootstrap script %s not found in %s with error %v, skipping",
			opts.BootstrapScript,
			baseDir,
			err,
		)
		return nil
	}

	//nolint:gosec // ensure executable, matching bootstrap.sh chmod +x
	if err := os.Chmod(scriptPath, 0o755); err != nil {
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
	repoPath := filepath.Join(opts.BootstrapDir, bootstrapRepoDirname)
	log.Infof("Cloning bootstrap repository %s (ref: %s) to %s", opts.RepoURL, opts.RepoRef, repoPath)

	if err := ensureSSHKnownHost(ctx, opts.RepoURL); err != nil {
		return err
	}

	// Remove any existing target directory so the clone is clean.
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to remove existing bootstrap dir %s: %w", repoPath, err)
	}

	//nolint:gosec // arguments come from CLI flags under user control
	cmd := exec.CommandContext(
		ctx,
		"git",
		"clone",
		"--depth",
		"1",
		"--branch",
		opts.RepoRef,
		opts.RepoURL,
		repoPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone of %s failed: %w", opts.RepoURL, err)
	}

	return nil
}

func ensureSSHKnownHost(ctx context.Context, repoURL string) error {
	sshServer := extractDomain(repoURL)
	if sshServer == "" {
		log.Warnf(
			"Failed to extract SSH server from repo URL %s, SSH host key will not be added to known_hosts",
			repoURL,
		)
		return nil
	}

	log.Infof("Waiting for SSH server %s to be resolvable...", sshServer)
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		addrs, err := net.DefaultResolver.LookupHost(ctx, sshServer)
		if err != nil {
			log.WithError(err).Debugf("SSH server %s not yet resolvable, retrying...", sshServer)
			return false, nil
		}
		if len(addrs) == 0 {
			log.Debugf("SSH server %s resolved without addresses, retrying...", sshServer)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("SSH server %s did not become resolvable: %w", sshServer, err)
	}

	log.Infof("Adding SSH server %s to known_hosts", sshServer)
	//nolint:gosec // sshServer is derived from repoURL and only used for ssh-keyscan host lookup.
	sshKeyscanCmd := exec.CommandContext(ctx, "ssh-keyscan", "-t", "rsa", sshServer)
	keyscanOutput, err := sshKeyscanCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to scan SSH key for %s: %w", sshServer, err)
	}

	// Ensure $HOME/.ssh directory exists so that ssh-keyscan can write to known_hosts without permission issues.
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")

	//nolint:gosec // path is constrained to user-local HOME and mode is intentionally restrictive.
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")

	//nolint:gosec // path is constrained to user-local HOME and mode is intentionally restrictive.
	if err := os.WriteFile(knownHostsPath, keyscanOutput, 0o600); err != nil {
		return fmt.Errorf("failed to write known_hosts file: %w", err)
	}

	return nil
}

func extractDomain(repoURL string) string {
	// This is a very naive extraction that assumes the repo URL is in the form "git@domain:owner/repo.git".
	// In a real implementation, you would want to handle more cases and validate the input properly.
	parts := strings.Split(repoURL, "@")
	if len(parts) != 2 {
		return ""
	}
	domainParts := strings.Split(parts[1], ":")
	if len(domainParts) != 2 {
		return ""
	}
	return domainParts[0]
}
