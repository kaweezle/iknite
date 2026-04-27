// cSpell: words kstatus sirupsen apimachinery testutil clientcmd
package kubewait

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	pollingEvent "sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/testutil"
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
				client := k8s.NewClientFromConfig(api.NewConfig())
				clientset, err := k8s.ClientSet(client)
				if err != nil {
					return fmt.Errorf("failed to create clientset: %w", err)
				}
				_, err = listNamespaces(context.Background(), clientset, time.Millisecond)
				return err
			},
		},
		{
			name: "wait namespace resources with invalid kubeconfig",
			run: func() error {
				client := k8s.NewClientFromConfig(api.NewConfig())
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
				fs := host.NewMemMapFS()
				return waitForResources(context.Background(), fs, opts, []string{"default"})
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

		fs := host.NewMemMapFS()
		h, err := testutil.NewDummyHost(fs, &testutil.DummyHostOptions{})
		req.NoError(err)

		err = RunKubewait(context.Background(), h, opts, []string{"default"})
		req.Error(err)
		req.Contains(err.Error(), "error while waiting for resources")
	})

	t.Run("wraps bootstrap error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		script := filepath.Join(bootstrapDir, "iknite-bootstrap.sh")
		req.NoError(fs.WriteFile(script, []byte("#!/bin/sh\nexit 7\n"), 0o600))
		req.NoError(fs.Chmod(script, 0o755))

		fakeExecs := map[string]*testutil.FakeProcessOutput{
			`/base/iknite-bootstrap\.sh`: testutil.FakeExec("", 7),
		}
		h := &testutil.DelegateHost{Fs: fs, Exec: testutil.NewDummyExecutor(map[int]host.Process{}, fakeExecs)}

		opts := NewOptions()
		opts.SkipWaitingForResources = true
		opts.SkipBootstrap = false
		opts.BootstrapDir = bootstrapDir
		opts.BootstrapScript = filepath.Base(script)

		// We need that as we cannot execute RAM based scripts (should mock execution in that case)
		err := RunKubewait(context.Background(), h, opts, []string{"default"})
		req.Error(err)
		req.Contains(err.Error(), "error during bootstrap")
	})
}
