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

// cSpell: words godotenv clientcmd apimachinery kstatus errorf sirupsen joho metav1

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	defaultBootstrapScript         = "iknite-bootstrap.sh"
	defaultBootstrapDir            = "/workspace/bootstrap-repo"
	defaultTimeout                 = 10 * time.Minute
	defaultStatusUpdateInterval    = 2 * time.Second
	defaultResourcesUpdateInterval = 2 * time.Second
	defaultSettlePeriod            = 6 * time.Second
	defaultNamespaceSettlePeriod   = 30 * time.Second
	defaultResourceTypes           = "deployments,statefulsets,daemonsets,jobs,cronjobs,applications"
)

// Options holds the configuration for the kubewait command.
type Options struct {
	Kubeconfig              string
	BootstrapDir            string
	BootstrapScript         string
	RepoURL                 string
	RepoRef                 string
	EnvFile                 string
	ResourceTypes           string
	Verbosity               string
	Timeout                 time.Duration
	StatusUpdateInterval    time.Duration
	ResourcesUpdateInterval time.Duration
	SettlePeriod            time.Duration
	NamespaceSettlePeriod   time.Duration
	JSONLogs                bool
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
	flags.DurationVar(&opts.StatusUpdateInterval, "status-update-interval", defaultStatusUpdateInterval,
		"Polling interval between readiness checks")
	flags.DurationVar(&opts.ResourcesUpdateInterval, "resources-update-interval", defaultResourcesUpdateInterval,
		"Polling interval between checks for new or deleted resources in the watched namespaces")
	flags.DurationVar(&opts.SettlePeriod, "settle-period", defaultSettlePeriod,
		"Time to wait after all workloads are ready before running the bootstrap script")
	flags.DurationVar(&opts.NamespaceSettlePeriod, "namespace-settle-period", defaultNamespaceSettlePeriod,
		"Grace period to wait after a namespace appears before checking its workloads")
	flags.StringVarP(&opts.Verbosity, "verbosity", "v", "info",
		"Log level (debug, info, warn, error, fatal, panic)")
	flags.BoolVar(&opts.JSONLogs, "json", false, "Emit log messages as JSON")
	flags.StringVar(
		&opts.ResourceTypes,
		"resource-types",
		defaultResourceTypes,
		//nolint:lll // flag description is long but it's helpful to list the supported resource types here
		"Comma-separated list of resource types to check (e.g. deployments,statefulsets,daemonsets,jobs,cronjobs,applications)",
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

	// Validate that the requested resource types are supported by the cluster before starting the wait loops.
	validTypes, err := client.ValidateResourceTypes(opts.ResourceTypes)
	if err != nil {
		return fmt.Errorf("resource type validation failed: %w", err)
	}
	if len(validTypes) == 0 {
		return errors.New("none of the specified resource types are supported by the cluster")
	}
	opts.ResourceTypes = strings.Join(validTypes, ",")

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
			if err := waitNamespaceWorkloads(ctx, client, k8sClient, ns, opts); err != nil {
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
	logger          log.FieldLogger
	client          *k8s.RESTClientGetter
	poller          *polling.StatusPoller
	settleTimer     *time.Timer
	allReadyChannel chan struct{}
	done            <-chan collector.ListenerResult
	opts            *Options
	namespace       string
	currentDataSet  object.ObjMetadataSet
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
		client:          client,
		namespace:       namespace,
		poller:          poller,
		logger:          logger,
		settleTimer:     nil,
		allReadyChannel: make(chan struct{}),
		done:            nil,
		currentDataSet:  object.ObjMetadataSet{},
		opts:            opts,
	}, nil
}

func (w *resourceWaiter) StartSettleTimer() error {
	if w.settleTimer != nil {
		return fmt.Errorf("settle timer already started")
	}
	w.settleTimer = time.AfterFunc(w.opts.SettlePeriod, func() {
		close(w.allReadyChannel)
	})

	return nil
}

func (w *resourceWaiter) StopSettleTimer() error {
	if w.settleTimer == nil {
		return fmt.Errorf("settle timer not started")
	}
	if !w.settleTimer.Stop() {
		<-w.settleTimer.C
	}
	w.settleTimer = nil
	return nil
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
	if aggStatus == status.CurrentStatus && w.settleTimer == nil {
		w.logger.WithField("timer", w.opts.SettlePeriod.Round(time.Second)).Info(
			"All workloads are ready, starting settle timer",
		)
		if err := w.StartSettleTimer(); err != nil {
			w.logger.Errorf("Failed to start settle timer: %v", err)
		}
	} else if aggStatus != status.CurrentStatus && w.settleTimer != nil {
		w.logger.Infof("A workload is no longer ready, stopping settle timer")
		if err := w.StopSettleTimer(); err != nil {
			w.logger.Errorf("Failed to stop settle timer: %v", err)
		}
	}
}

func (w *resourceWaiter) Start(ctx context.Context) error {
	var err error
	w.currentDataSet, err = w.client.ObjectMetadataSetForNamespace(w.namespace, w.opts.ResourceTypes)
	if err != nil {
		return fmt.Errorf("failed to get object metadata set for namespace %s: %w", w.namespace, err)
	}

	if len(w.currentDataSet) == 0 {
		w.logger.Info("No workloads found in namespace, waiting for settle period")
		if err := w.StartSettleTimer(); err != nil {
			return fmt.Errorf("failed to start settle timer: %w", err)
		}
	} else {
		coll := collector.NewResourceStatusCollector(w.currentDataSet)

		eventChannel := w.poller.Poll(ctx, w.currentDataSet, polling.PollOptions{
			PollInterval: w.opts.StatusUpdateInterval,
		})

		w.done = coll.ListenWithObserver(eventChannel, collector.ObserverFunc(
			func(rsc *collector.ResourceStatusCollector, event pollingEvent.Event) {
				w.processEvent(rsc, event)
			}),
		)
	}

	return nil
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

	logger.Infof(
		"Waiting for workloads to become ready with a status update interval of %s...",
		opts.StatusUpdateInterval.Round(time.Second),
	)
	for {
		select {
		case result := <-waiter.done:
			if result.Err != nil {
				return fmt.Errorf("error while polling workloads in namespace %s: %w", namespace, result.Err)
			}
			return nil
		case <-cancellableCtx.Done():
			return fmt.Errorf("context canceled while polling workloads in namespace %s: %w", namespace, ctx.Err())
		case <-waiter.allReadyChannel:
			// This case is hit when the settle timer completes,
			// which means all workloads have been ready for the entire settle period.
			logger.Info("Namespace ready")
			return nil
		}
	}
}

// NamespaceWorkloadCondition returns a ConditionWithContextFunc that polls workload readiness
// for a single namespace.
func NamespaceWorkloadCondition(
	client *k8s.RESTClientGetter,
	namespace string,
	resourceTypes string,
) wait.ConditionWithContextFunc {
	return func(_ context.Context) (bool, error) {
		states, err := client.WorkloadStatesForNamespace(namespace, resourceTypes)
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
		log.Infof(
			"Bootstrap script %s not found in %s with error %v, skipping",
			opts.BootstrapScript,
			opts.BootstrapDir,
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
	log.Infof("Cloning bootstrap repository %s (ref: %s) to %s", opts.RepoURL, opts.RepoRef, opts.BootstrapDir)

	// Remove any existing target directory so the clone is clean.
	if err := os.RemoveAll(opts.BootstrapDir); err != nil {
		return fmt.Errorf("failed to remove existing bootstrap dir %s: %w", opts.BootstrapDir, err)
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
		opts.BootstrapDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone of %s failed: %w", opts.RepoURL, err)
	}

	return nil
}
