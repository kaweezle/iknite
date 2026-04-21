// cSpell: words kstatus sirupsen apimachinery
package kubewait

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	pollingEvent "sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/kaweezle/iknite/pkg/k8s"
)

func TestClientDependentHelpersReturnErrors(t *testing.T) {
	t.Parallel()

	missingKubeconfig := filepath.Join(t.TempDir(), "missing.conf")

	tests := []struct {
		run  func() error
		name string
	}{
		{
			name: "list namespaces with invalid kubeconfig",
			run: func() error {
				client := k8s.NewClientFromKubeconfig(missingKubeconfig)
				_, err := listNamespaces(context.Background(), client, time.Millisecond)
				return err
			},
		},
		{
			name: "wait namespace resources with invalid kubeconfig",
			run: func() error {
				client := k8s.NewClientFromKubeconfig(missingKubeconfig)
				opts := NewOptions()
				opts.StatusUpdateInterval = time.Millisecond
				return waitNamespaceResources(context.Background(), client, "default", opts)
			},
		},
		{
			name: "wait for resources with invalid kubeconfig",
			run: func() error {
				opts := NewOptions()
				opts.Kubeconfig = missingKubeconfig
				opts.ResourceTypes = []string{"deployments"}
				opts.StatusUpdateInterval = time.Millisecond
				return waitForResources(context.Background(), opts, []string{"default"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			err := tt.run()
			req.Error(err)
		})
	}
}

func TestResourceWaiterCancelAndTimerHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	watchCanceled := false
	pollCanceled := false

	w := &resourceWaiter{
		logger:     log.New(),
		opts:       &Options{ResourcesOptions: ResourcesOptions{SettlePeriod: 10 * time.Millisecond}},
		endChannel: make(chan error, 1),
	}
	w.watchDatasetCancel = func() { watchCanceled = true }
	w.pollCancel = func() { pollCanceled = true }

	w.stopWatchingDataSetChanges()
	req.True(watchCanceled)
	req.Nil(w.watchDatasetCancel)

	w.stopPolling()
	req.True(pollCanceled)
	req.Nil(w.pollCancel)

	req.NoError(w.StartSettleTimer())
	select {
	case err := <-w.endChannel:
		req.NoError(err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for settle timer callback")
	}
	req.NoError(w.StopSettleTimer())
	req.False(w.hasSettleTimer())
}

func TestResourceWaiterProcessEventStartsAndStopsSettleTimer(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	w := &resourceWaiter{
		logger:     log.New(),
		opts:       &Options{ResourcesOptions: ResourcesOptions{SettlePeriod: time.Second}},
		endChannel: make(chan error, 1),
	}

	resourceID := object.ObjMetadata{
		GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
		Namespace: "default",
		Name:      "demo",
	}

	rsc := &collector.ResourceStatusCollector{
		ResourceStatuses: map[object.ObjMetadata]*pollingEvent.ResourceStatus{
			resourceID: {Identifier: resourceID, Status: status.CurrentStatus, Message: "ready"},
		},
	}

	w.processEvent(
		rsc,
		pollingEvent.Event{Type: pollingEvent.ResourceUpdateEvent, Resource: rsc.ResourceStatuses[resourceID]},
	)
	req.True(w.hasSettleTimer())

	rsc.ResourceStatuses[resourceID].Status = status.InProgressStatus
	w.processEvent(
		rsc,
		pollingEvent.Event{Type: pollingEvent.ResourceUpdateEvent, Resource: rsc.ResourceStatuses[resourceID]},
	)
	req.False(w.hasSettleTimer())
}

func TestRunKubewaitErrorWrapping(t *testing.T) {
	t.Parallel()

	t.Run("wraps wait error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts := NewOptions()
		opts.Kubeconfig = filepath.Join(t.TempDir(), "missing.conf")
		opts.ResourceTypes = []string{"deployments"}
		opts.StatusUpdateInterval = time.Millisecond
		opts.SkipBootstrap = true

		err := RunKubewait(context.Background(), opts, []string{"default"})
		req.Error(err)
		req.Contains(err.Error(), "error while waiting for resources")
	})

	t.Run("wraps bootstrap error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		dir := t.TempDir()
		script := filepath.Join(dir, "iknite-bootstrap.sh")
		req.NoError(os.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0o600))
		req.NoError(os.Chmod(script, 0o755)) //nolint:gosec // test script needs executable bit

		opts := NewOptions()
		opts.SkipWaitingForResources = true
		opts.SkipBootstrap = false
		opts.BootstrapDir = dir
		opts.BootstrapScript = filepath.Base(script)

		err := RunKubewait(context.Background(), opts, []string{"default"})
		req.Error(err)
		req.Contains(err.Error(), "error during bootstrap")
	})
}
