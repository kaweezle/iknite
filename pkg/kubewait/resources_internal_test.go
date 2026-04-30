// cSpell: words kstatus sirupsen apimachinery testutil clientcmd genericclioptions corev metav unknownresource
//
//nolint:gocognit // Complex test cases.
package kubewait

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliOptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	pollingEvent "sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"

	mockCliOptions "github.com/kaweezle/iknite/mocks/k8s.io/cli-runtime/pkg/genericclioptions"
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
				opts := NewResourcesOptions()
				opts.StatusUpdateInterval = time.Millisecond
				return waitNamespaceResources(context.Background(), client, "default", opts)
			},
		},
		{
			name: "wait for resources with invalid kubeconfig",
			run: func() error {
				opts := NewResourcesOptions()
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
		logger:     logrus.New(),
		opts:       &ResourcesOptions{SettlePeriod: 10 * time.Millisecond},
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
		logger:     logrus.New(),
		opts:       &ResourcesOptions{SettlePeriod: time.Second},
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

func Test_listNamespaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		createGetter func(t *testing.T, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter
		assertions   func(req *require.Assertions, got []string, opts *testutil.TestServerOptions)
		name         string
		wantErr      string
		want         []string
		interval     time.Duration
		timeout      time.Duration
	}{
		{
			name: "returns default namespace",
			createGetter: func(t *testing.T, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			want:     []string{"default"},
			wantErr:  "",
			assertions: func(req *require.Assertions, _ []string, opts *testutil.TestServerOptions) {
				req.Len(opts.Requests, 1)
				req.Equal("/api/v1/namespaces", opts.Requests[0].Path)
			},
		},
		{
			name: "times out if server does not respond",
			createGetter: func(t *testing.T, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				opts.Overrides = map[string]testutil.HandlerOverrideFunc{
					"/api/v1/namespaces": testutil.FailOverrideHandler,
				}
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			timeout:  20 * time.Millisecond,
			want:     nil,
			wantErr:  "context deadline exceeded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			serverOpts := &testutil.TestServerOptions{}
			getter := tt.createGetter(t, serverOpts)
			k8sInterface, err := k8s.ClientSet(getter)
			req.NoError(err, "failed to create Kubernetes clientset")
			ctx := context.Background()
			if tt.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}

			got, gotErr := listNamespaces(ctx, k8sInterface, tt.interval)
			if gotErr != nil {
				if tt.wantErr == "" {
					req.NoError(gotErr, "listNamespaces() returned unexpected error")
				}
				req.Contains(gotErr.Error(), tt.wantErr, "listNamespaces() error does not contain expected message")
				return
			}
			req.Equal(tt.want, got, "Did not get expected namespaces")
			if tt.assertions != nil {
				tt.assertions(req, got, serverOpts)
			}
		})
	}
}

func Test_resolveNamespaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		createGetter  func(t *testing.T, fs host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter
		assertions    func(req *require.Assertions, got []string, opts *testutil.TestServerOptions)
		name          string
		wantErr       string
		namespaces    []string
		want          []string
		interval      time.Duration
		timeout       time.Duration
		allNamespaces bool
	}{
		{
			name:       "returns provided namespaces without querying API server",
			namespaces: []string{"ns1", "ns2"},
			createGetter: func(t *testing.T, _ host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			want:     []string{"ns1", "ns2"},
			wantErr:  "",
			assertions: func(req *require.Assertions, _ []string, opts *testutil.TestServerOptions) {
				req.Empty(opts.Requests)
			},
		},
		{
			name: "returns default namespace",
			createGetter: func(t *testing.T, _ host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			want:     []string{"default"},
			wantErr:  "",
			assertions: func(req *require.Assertions, _ []string, opts *testutil.TestServerOptions) {
				req.Len(opts.Requests, 1)
				req.Equal("/api/v1/namespaces", opts.Requests[0].Path)
			},
		},
		{
			name: "returns all namespaces",
			createGetter: func(t *testing.T, fs host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				require.NoError(t, fs.WriteFile(currentNamespaceFile, []byte("other"), os.FileMode(0o644)))
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval:      10 * time.Millisecond,
			allNamespaces: true,
			want:          []string{"default"},
			wantErr:       "",
			assertions: func(req *require.Assertions, _ []string, opts *testutil.TestServerOptions) {
				req.Len(opts.Requests, 1)
				req.Equal("/api/v1/namespaces", opts.Requests[0].Path)
			},
		},
		{
			name: "times out if server does not respond",
			createGetter: func(t *testing.T, _ host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				opts.Overrides = map[string]testutil.HandlerOverrideFunc{
					"/api/v1/namespaces": testutil.FailOverrideHandler,
				}
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			timeout:  20 * time.Millisecond,
			want:     nil,
			wantErr:  "context deadline exceeded",
		},
		{
			name: "returns local namespace",
			createGetter: func(t *testing.T, fs host.FileSystem, opts *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				require.NoError(t, fs.WriteFile(currentNamespaceFile, []byte("other"), os.FileMode(0o644)))
				return testutil.CreateDefaultTestClientGetter(t, opts)
			},
			interval: 10 * time.Millisecond,
			want:     []string{"other"},
			wantErr:  "",
			assertions: func(req *require.Assertions, _ []string, opts *testutil.TestServerOptions) {
				req.Empty(opts.Requests)
			},
		},
		{
			name: "fails to create getter",
			createGetter: func(t *testing.T, _ host.FileSystem, _ *testutil.TestServerOptions) cliOptions.RESTClientGetter {
				t.Helper()
				getter := mockCliOptions.NewMockRESTClientGetter(t)
				getter.EXPECT().ToRESTMapper().Return(testutil.NewRESTMapper(), nil).Maybe()
				getter.EXPECT().ToRESTConfig().Return(nil, errors.New("bad config")).Once()
				return getter
			},
			interval: 10 * time.Millisecond,
			wantErr:  "failed to create Kubernetes client",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			serverOpts := &testutil.TestServerOptions{}
			fs := host.NewMemMapFS()
			getter := tt.createGetter(t, fs, serverOpts)
			ctx := context.Background()
			if tt.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}
			opts := NewResourcesOptions()
			opts.StatusUpdateInterval = tt.interval
			opts.AllNamespaces = tt.allNamespaces

			got, gotErr := resolveNamespaces(ctx, getter, fs, opts, tt.namespaces)
			if gotErr != nil {
				if tt.wantErr == "" {
					req.NoError(gotErr, "resolveNamespaces() returned unexpected error")
				}
				req.Contains(gotErr.Error(), tt.wantErr, "resolveNamespaces() error does not contain expected message")
				return
			}
			req.Equal(tt.want, got, "Did not get expected namespaces")
			if tt.assertions != nil {
				tt.assertions(req, got, serverOpts)
			}
		})
	}
}

func TestNewResourceWaiter(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	serverOpts := &testutil.TestServerOptions{}
	okGetter := testutil.CreateDefaultTestClientGetter(t, serverOpts)

	opts := NewResourcesOptions()

	waiter, err := newResourceWaiter(okGetter, "default", opts)
	req.NoError(err)
	req.NotNil(waiter)
	req.NotNil(waiter.client)
	req.NotNil(waiter.poller)

	// Now create a bad getter
	getter := mockCliOptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(testutil.NewRESTMapper(), nil).Maybe()
	getter.EXPECT().ToRESTConfig().Return(nil, errors.New("bad config")).Once()
	waiter, err = newResourceWaiter(getter, "default", opts)
	req.Error(err)
	req.Nil(waiter)
}

const (
	defaultNamespacePath = "/api/v1/namespaces/default"
	deploymentsPath      = "/apis/apps/v1/namespaces/default/deployments"
)

func TestWaitNameSpaceResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		createGetter func(t *testing.T, sOpts *testutil.TestServerOptions,
			rOpts *ResourcesOptions, cancel context.CancelFunc) cliOptions.RESTClientGetter
		assertions func(req *require.Assertions, opts *testutil.TestServerOptions)
		name       string
		namespace  string
		wantErr    string
		timeout    time.Duration
	}{
		{
			name:      "nominal case, everything is ready",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "",
			assertions: func(req *require.Assertions, opts *testutil.TestServerOptions) {
				opts.RequestMu.Lock()
				defer opts.RequestMu.Unlock()
				req.Equal(defaultNamespacePath, opts.Requests[0].Path)
			},
		},
		{
			name:      "Namespace takes some time to appear but eventually does",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				counter := 0
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					defaultNamespacePath: func(path string, w http.ResponseWriter, r *http.Request,
						_ *testutil.RequestLog, _ embed.FS,
					) bool {
						counter++
						if counter < 3 {
							logrus.Infof("Simulating delayed namespace creation for path: %s", path)
							http.NotFound(w, r)
							return true
						}
						return false
					},
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "",
			assertions: func(req *require.Assertions, opts *testutil.TestServerOptions) {
				opts.RequestMu.Lock()
				defer opts.RequestMu.Unlock()
				req.Equal(defaultNamespacePath, opts.Requests[0].Path)
				req.Equal(defaultNamespacePath, opts.Requests[1].Path)
			},
		},
		{
			name:      "fails to create clientset",
			namespace: "default",
			createGetter: func(t *testing.T, _ *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				getter := mockCliOptions.NewMockRESTClientGetter(t)
				getter.EXPECT().ToRESTMapper().Return(testutil.NewRESTMapper(), nil).Maybe()
				getter.EXPECT().ToRESTConfig().Return(nil, errors.New("bad config")).Once()
				return getter
			},
			timeout: 5 * time.Second,
			wantErr: "failed to create Kubernetes client",
		},
		{
			name:      "Fails to get namespaces",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					defaultNamespacePath: testutil.FailOverrideHandler,
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "namespace default did not appear",
		},
		{
			name:      "Fails to get resources to monitor",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					deploymentsPath: testutil.FailOverrideHandler,
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "failed to start resource waiter",
		},
		{
			name:      "Context fails while waiting for resources",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, cancel context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				counter := 0
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					deploymentsPath: func(path string, _ http.ResponseWriter,
						_ *http.Request, _ *testutil.RequestLog, _ embed.FS,
					) bool {
						counter++
						if counter == 1 {
							return false // Let the first request pass to start the waiter
						}
						logrus.Infof("Simulating context cancellation for path: %s", path)
						cancel() // Cancel the context to simulate timeout
						return false
					},
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			wantErr: "context canceled while polling resources",
		},
		{
			name:      "Namespace has just been created",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				counter := 0
				var timeOfFirstRequest time.Time
				var content []byte
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					defaultNamespacePath: func(_ string, w http.ResponseWriter, _ *http.Request,
						log *testutil.RequestLog, fs embed.FS,
					) bool {
						counter++
						if counter == 1 {
							timeOfFirstRequest = time.Now()
							var err error
							content, err = fs.ReadFile("testdata/with_resources" + defaultNamespacePath + ".json")
							if err != nil {
								logrus.Errorf("Failed to read namespace fixture: %v", err)
								http.Error(w, "Internal Server Error", http.StatusInternalServerError)
								log.StatusCode = http.StatusInternalServerError
								return true
							}
							namespace := &corev1.Namespace{}
							if err = json.Unmarshal(content, namespace); err != nil {
								logrus.Errorf("Failed to unmarshal namespace fixture: %v", err)
								http.Error(w, "Internal Server Error", http.StatusInternalServerError)
								log.StatusCode = http.StatusInternalServerError
								return true
							}
							namespace.CreationTimestamp = metav1.NewTime(timeOfFirstRequest)
							if content, err = json.Marshal(namespace); err != nil {
								logrus.Errorf("Failed to marshal namespace fixture: %v", err)
							}
							logrus.Infof(
								"Simulating namespace creation at: %s",
								timeOfFirstRequest.Format(time.RFC3339),
							)
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						log.StatusCode = http.StatusOK
						_, _ = w.Write(content) //nolint:errcheck // test server, ignore error

						return true
					},
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "",
			assertions: func(req *require.Assertions, opts *testutil.TestServerOptions) {
				opts.RequestMu.Lock()
				defer opts.RequestMu.Unlock()
				req.Equal(defaultNamespacePath, opts.Requests[0].Path)
			},
		},
		{
			name:      "Fails to get resources to monitor",
			namespace: "default",
			createGetter: func(t *testing.T, sOpts *testutil.TestServerOptions,
				_ *ResourcesOptions, _ context.CancelFunc,
			) cliOptions.RESTClientGetter {
				t.Helper()
				counter := 0
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					deploymentsPath: func(path string, w http.ResponseWriter, _ *http.Request,
						log *testutil.RequestLog, _ embed.FS,
					) bool {
						counter++
						if counter == 1 {
							return false // Let the first request pass to start the waiter
						}
						logrus.Infof("Simulating failure to get resources for path: %s", path)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						log.StatusCode = http.StatusInternalServerError
						return true
					},
				}
				return testutil.CreateDefaultTestClientGetter(t, sOpts)
			},
			timeout: 5 * time.Second,
			wantErr: "error while polling resources in namespace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			serverOpts := &testutil.TestServerOptions{}
			// Setting short timeouts
			rOpts := &ResourcesOptions{
				ResourceTypes:           []string{"deployments", "statefulsets", "daemonsets"},
				StatusUpdateInterval:    10 * time.Millisecond,
				ResourcesUpdateInterval: 10 * time.Millisecond,
				SettlePeriod:            10 * time.Millisecond,
				NamespaceSettlePeriod:   1 * time.Second,
			}
			ctx := t.Context()
			var cancel context.CancelFunc
			if tt.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()
			getter := tt.createGetter(t, serverOpts, rOpts, cancel)

			gotErr := waitNamespaceResources(ctx, getter, tt.namespace, rOpts)
			if tt.wantErr != "" {
				req.Error(gotErr)
				req.Contains(
					gotErr.Error(),
					tt.wantErr, "waitNamespaceResources() error does not contain expected message",
				)
				return
			}
			req.NoError(gotErr)
			if tt.assertions != nil {
				tt.assertions(req, serverOpts)
			}
		})
	}
}

func Test_waitForResources(t *testing.T) {
	t.Parallel()

	const kubeconfigPath = "/kubeconfig"

	tests := []struct {
		overrides     map[string]testutil.HandlerOverrideFunc
		name          string
		wantErr       string
		mapperType    string
		resourceTypes []string
		namespaces    []string
		timeout       time.Duration
		allNamespaces bool
	}{
		{
			name:       "succeeds with explicit namespace and timeout",
			namespaces: []string{"default"},
			timeout:    5 * time.Second,
		},
		{
			name:       "succeeds with zero timeout",
			namespaces: []string{"default"},
			timeout:    0,
		},
		{
			name:          "fails when no resource types are valid",
			resourceTypes: []string{"unknownresource"},
			namespaces:    []string{"default"},
			wantErr:       "none of the specified resource types are supported by the cluster",
		},
		{
			name:          "fails when namespace listing times out",
			allNamespaces: true,
			timeout:       50 * time.Millisecond,
			overrides: map[string]testutil.HandlerOverrideFunc{
				"/api/v1/namespaces": testutil.FailOverrideHandler,
			},
			wantErr: "failed to list namespaces",
		},
		{
			name:       "fails when namespace does not appear",
			namespaces: []string{"default"},
			timeout:    5 * time.Second,
			overrides: map[string]testutil.HandlerOverrideFunc{
				defaultNamespacePath: testutil.FailOverrideHandler,
			},
			wantErr: "namespace default did not appear",
		},
		{
			name:          "fails when no resource types are valid",
			resourceTypes: []string{"unknownresource"},
			namespaces:    []string{"default"},
			mapperType:    "mock",
			wantErr:       "resource type validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			sOpts := &testutil.TestServerOptions{Overrides: tt.overrides}
			fs := host.NewMemMapFS()
			restConfig := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
			mapperType := tt.mapperType
			if mapperType == "" {
				mapperType = "static"
			}
			testutil.WriteRestConfigToFile(t, restConfig, fs, kubeconfigPath, "test", mapperType)

			resourceTypes := tt.resourceTypes
			if len(resourceTypes) == 0 {
				resourceTypes = []string{"deployments"}
			}

			rOpts := &ResourcesOptions{
				Kubeconfig:              kubeconfigPath,
				ResourceTypes:           resourceTypes,
				AllNamespaces:           tt.allNamespaces,
				Timeout:                 tt.timeout,
				StatusUpdateInterval:    10 * time.Millisecond,
				ResourcesUpdateInterval: 10 * time.Millisecond,
				SettlePeriod:            10 * time.Millisecond,
				NamespaceSettlePeriod:   1 * time.Second,
			}

			gotErr := waitForResources(t.Context(), fs, rOpts, tt.namespaces)
			if tt.wantErr != "" {
				req.Error(gotErr)
				req.Contains(gotErr.Error(), tt.wantErr)
				return
			}
			req.NoError(gotErr)
		})
	}
}
