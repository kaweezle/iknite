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

// cSpell: words godotenv clientcmd apimachinery kstatus errorf sirupsen joho metav1 serviceaccount genericclioptions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	coreV1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	defaultTimeout                 = 10 * time.Minute
	defaultStatusUpdateInterval    = 4 * time.Second
	defaultResourcesUpdateInterval = 2 * time.Second
	defaultSettlePeriod            = 10 * time.Second
	defaultNamespaceSettlePeriod   = 20 * time.Second
	currentNamespaceFile           = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

var defaultResourceTypes = []string{"deployments", "statefulsets", "daemonsets", "jobs", "cronjobs", "applications"}

// ResourcesOptions holds the configuration for the kubewait command.
type ResourcesOptions struct {
	Kubeconfig              string
	ResourceTypes           []string
	Timeout                 time.Duration
	StatusUpdateInterval    time.Duration
	ResourcesUpdateInterval time.Duration
	SettlePeriod            time.Duration
	NamespaceSettlePeriod   time.Duration
}

func NewResourcesOptions() ResourcesOptions {
	return ResourcesOptions{
		Timeout:                 defaultTimeout,
		StatusUpdateInterval:    defaultStatusUpdateInterval,
		ResourcesUpdateInterval: defaultResourcesUpdateInterval,
		SettlePeriod:            defaultSettlePeriod,
		NamespaceSettlePeriod:   defaultNamespaceSettlePeriod,
	}
}

func AddResourcesFlags(flags *pflag.FlagSet, opts *ResourcesOptions) {
	flags.StringVar(&opts.Kubeconfig, "kubeconfig", "",
		"Path to kubeconfig file (defaults to KUBECONFIG env var or ~/.kube/config; falls back to in-cluster config)")
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
	flags.StringSliceVar(
		&opts.ResourceTypes,
		"resource-types",
		defaultResourceTypes,
		"Comma-separated list of resource types to check",
	)
}

// waitForResources waits for all resources in the specified namespaces to become ready.
func waitForResources(ctx context.Context, opts *Options, namespaces []string) error {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	client := k8s.NewClientFromKubeconfig(opts.Kubeconfig)

	mapper, err := client.ToRESTMapper()
	if err != nil {
		return fmt.Errorf("failed to create REST mapper: %w", err)
	}

	// Validate that the requested resource types are supported by the cluster before starting the wait loops.
	validTypes, err := k8s.ValidateResourceTypes(mapper, opts.ResourceTypes)
	if err != nil {
		return fmt.Errorf("resource type validation failed: %w", err)
	}
	if len(validTypes) == 0 {
		return errors.New("none of the specified resource types are supported by the cluster")
	}
	opts.ResourceTypes = validTypes

	// If no namespaces were given, list all that exist right now.
	if len(namespaces) == 0 { //nolint:nestif // this is clearer as a single if block
		if info, err := os.Stat(currentNamespaceFile); err == nil && !info.IsDir() && !opts.AllNamespaces {
			log.Infof("Getting namespace from %s", currentNamespaceFile)
			namespaceBytes, readErr := os.ReadFile(currentNamespaceFile)
			if readErr != nil {
				return fmt.Errorf("failed to read namespace from file %s: %w", currentNamespaceFile, readErr)
			}
			namespace := string(namespaceBytes)
			log.Infof("Watching current namespace: %s", namespace)
			namespaces = []string{namespace}
		} else {
			k8sInterface, err := k8s.ClientSet(client)
			if err != nil {
				return fmt.Errorf("failed to create Kubernetes client: %w", err)
			}

			namespaces, err = listNamespaces(ctx, k8sInterface, opts.StatusUpdateInterval)
			if err != nil {
				return fmt.Errorf("failed to list namespaces: %w", err)
			}
		}
	}

	log.Infof("Watching %d namespace(s) concurrently: %v", len(namespaces), namespaces)

	// Launch one goroutine per namespace; collect errors via a buffered channel.
	errCh := make(chan error, len(namespaces))
	var wg sync.WaitGroup
	for _, ns := range namespaces {
		wg.Go(func() {
			if err := waitNamespaceResources(ctx, client, ns, opts); err != nil {
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

// listNamespaces polls the API server until it can list all namespaces.
func listNamespaces(
	ctx context.Context,
	k8sInterface kubernetes.Interface,
	interval time.Duration,
) ([]string, error) {
	log.Info("No namespaces specified, listing all namespaces from the cluster...")

	var names []string
	if err := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		list, listErr := k8sInterface.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
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
	client             genericclioptions.RESTClientGetter
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
	client genericclioptions.RESTClientGetter,
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
	dataSet, err := k8s.ObjectMetadataSetForNamespace(w.client, w.namespace, w.opts.ResourceTypes)
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
	client genericclioptions.RESTClientGetter,
	namespace string,
	opts *Options,
) error {
	logger := log.WithField("namespace", namespace)
	// 1. Wait for the namespace to exist.
	logger.Infof("Waiting for namespace to exist...")
	var ns *coreV1.Namespace
	k8sInterface, err := k8s.ClientSet(client)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	if err = wait.PollUntilContextCancel(
		ctx,
		opts.StatusUpdateInterval,
		true,
		func(ctx context.Context) (bool, error) {
			var nsErr error
			ns, nsErr = k8sInterface.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if nsErr != nil {
				if k8errors.IsNotFound(nsErr) {
					logger.Debugf("Namespace not yet present, waiting...")
					return false, nil
				}
				return false, fmt.Errorf("error checking namespace: %w", nsErr)
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
