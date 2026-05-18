// cSpell: words clientset corev metav apimachinery errgroup genericclioptions lbip testutil errchkjson sirupsen
// cSpell: words paralleltest
package init

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	mockGenericCLI "github.com/kaweezle/iknite/mocks/k8s.io/cli-runtime/pkg/genericclioptions"
	mockV1 "github.com/kaweezle/iknite/mocks/k8s.io/client-go/kubernetes/typed/core/v1"
	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const (
	testOutboundIP = "10.196.248.109"
	testClusterIP  = "10.196.248.110"
	//nolint:lll // JSON string is long and difficult to break into multiple lines without losing readability
	serviceEvent = `{"type":"ADDED","object":{"kind":"Service","apiVersion":"v1","metadata":{"name":"argocd-gateway","namespace":"argocd","uid":"53bb5ca7-e030-41c0-9dd2-47d5acb43f6c","resourceVersion":"1065","creationTimestamp":"2026-05-10T16:14:31Z","labels":{"app.kubernetes.io/instance":"argocd-gateway","app.kubernetes.io/managed-by":"kgateway","app.kubernetes.io/name":"argocd-gateway","app.kubernetes.io/version":"v2.2.2"},"annotations":{"config.iknite.app/outbound-ip":"true"}},"spec":{"ports":[{"name":"listener-80","protocol":"TCP","port":80,"targetPort":80,"nodePort":30818},{"name":"listener-443","protocol":"TCP","port":443,"targetPort":443,"nodePort":32243}],"selector":{"app.kubernetes.io/instance":"argocd-gateway","app.kubernetes.io/name":"argocd-gateway","gateway.networking.k8s.io/gateway-name":"argocd-gateway"},"type":"LoadBalancer","sessionAffinity":"None","externalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","allocateLoadBalancerNodePorts":true,"internalTrafficPolicy":"Cluster"}}}`
)

func createMockHostWithOutboundIP(t *testing.T, ip string) *mockHost.MockHost {
	t.Helper()
	if ip == "" {
		ip = testOutboundIP
	}
	h := mockHost.NewMockHost(t)
	h.On("GetOutboundIP").Return(net.ParseIP(ip), nil)
	return h
}

func serviceToFillHandler(
	_ string,
	w http.ResponseWriter,
	_ *http.Request,
	log *testutil.RequestLog,
	_ embed.FS,
	_ *slog.Logger,
) bool {
	w.Header().Set("Content-Type", "application/json")
	log.StatusCode = http.StatusOK
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(serviceEvent)) //nolint:errcheck // In tests we can ignore write errors

	return true
}

func toUpdateServiceOptions() *testutil.TestServerOptions {
	return &testutil.TestServerOptions{
		Overrides: map[string]testutil.HandlerOverrideFunc{
			"/api/v1/services": serviceToFillHandler,
		},
	}
}

func createGetter(t *testing.T, sOpts *testutil.TestServerOptions) genericclioptions.RESTClientGetter {
	t.Helper()
	return testutil.CreateDefaultTestClientGetter(t, sOpts)
}

//nolint:containedctx // context is provided by the workflow.RunData
type setLBIPPhaseData struct {
	host     *mockHost.MockHost
	ctx      context.Context
	cluster  *v1alpha1.IkniteCluster
	getter   genericclioptions.RESTClientGetter
	errGroup *errgroup.Group
	logger   *slog.Logger
}

var _ setLBIPData = (*setLBIPPhaseData)(nil)

func (d *setLBIPPhaseData) Host() host.Host {
	return d.host
}

func (d *setLBIPPhaseData) Context() context.Context {
	return d.ctx
}

func (d *setLBIPPhaseData) IkniteCluster() *v1alpha1.IkniteCluster {
	return d.cluster
}

func (d *setLBIPPhaseData) RESTClientGetter() (genericclioptions.RESTClientGetter, error) {
	if d.getter == nil {
		return nil, errors.New("no getter available")
	}
	return d.getter, nil
}

func (d *setLBIPPhaseData) Logger() *slog.Logger {
	return d.logger
}

func (d *setLBIPPhaseData) ErrGroup() *errgroup.Group {
	return d.errGroup
}

func TestNewSetLBIPPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	phase := NewSetLBIPPhase()
	req.Equal(setLBIPPhaseName, phase.Name)
	req.NotEmpty(phase.Short)
	req.NotNil(phase.Run)
}

func TestRunSetLBIP_FailsOnInvalidData(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	err := runSetLBIP("invalid-data")
	req.Error(err)
	req.Contains(err.Error(), "invalid data struct")
}

func TestRunSetLBIP_NominalCase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	sOpts := toUpdateServiceOptions()

	data := &setLBIPPhaseData{
		ctx:      t.Context(),
		errGroup: &errgroup.Group{},
		host:     createMockHostWithOutboundIP(t, ""),
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP(testOutboundIP)}},
		getter:   createGetter(t, sOpts),
		logger:   testutil.TestLogger(t),
	}

	req.NoError(runSetLBIP(data))
	req.NoError(data.errGroup.Wait())
	req.Len(sOpts.Requests, 2)
	writeLog := sOpts.Requests[1]
	req.Equal(http.MethodPut, writeLog.Method)
	req.Equal("/api/v1/namespaces/argocd/services/argocd-gateway/status", writeLog.Path)
}

func TestRunSetLBIP_UsesOutboundAndClusterIPWhenDifferent(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	sOpts := toUpdateServiceOptions()

	data := &setLBIPPhaseData{
		ctx:      t.Context(),
		errGroup: &errgroup.Group{},
		host:     createMockHostWithOutboundIP(t, testOutboundIP),
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP(testClusterIP)}},
		getter:   createGetter(t, sOpts),
		logger:   testutil.TestLogger(t),
	}

	req.NoError(runSetLBIP(data))
	req.NoError(data.errGroup.Wait())
	req.Len(sOpts.Requests, 2)

	writeLog := sOpts.Requests[1]
	req.Equal(http.MethodPut, writeLog.Method)
	req.Equal("/api/v1/namespaces/argocd/services/argocd-gateway/status", writeLog.Path)
	req.Contains(writeLog.Body, testOutboundIP)
	req.Contains(writeLog.Body, testClusterIP)
}

func TestRunSetLBIP_FailsOnRESTClientGetterError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	data := &setLBIPPhaseData{
		ctx:      t.Context(),
		getter:   nil,
		errGroup: &errgroup.Group{},
		host:     createMockHostWithOutboundIP(t, ""),
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP(testOutboundIP)}},
		logger:   testutil.TestLogger(t),
	}

	err := runSetLBIP(data)
	req.Error(err)
	req.Contains(err.Error(), "no getter available")
}

func TestShouldPatchServiceLBIP_WithValidAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{setLBIPAnnotation: setLBIPAnnotationValue},
		},
	}

	req.True(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestShouldPatchServiceLBIP_IgnoresNonLoadBalancer(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{setLBIPAnnotation: setLBIPAnnotationValue},
		},
	}

	req.False(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestShouldPatchServiceLBIP_IgnoresWrongAnnotationValue(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{setLBIPAnnotation: "false"},
		},
	}

	req.False(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestShouldPatchServiceLBIP_IgnoresMissingAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		ObjectMeta: metav1.ObjectMeta{},
	}

	req.False(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestRunSetLBIP_StartsWatcher(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockGetter := mockGenericCLI.NewMockRESTClientGetter(t)
	mockGetter.EXPECT().ToRESTConfig().Return(nil, errors.New("mock error")).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	data := &setLBIPPhaseData{
		ctx:      ctx,
		getter:   mockGetter,
		errGroup: eg,
		host:     createMockHostWithOutboundIP(t, ""),
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP(testOutboundIP)}},
		logger:   testutil.TestLogger(t),
	}

	// This will fail because the mock returns an error
	err := runSetLBIP(data)
	req.Error(err)
}

func TestSetLBIPAnnotationConstant(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	req.Equal("config.iknite.app/outbound-ip", setLBIPAnnotation)
}

func TestSetLBIPPhaseNameConstant(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	req.Equal("set-lb-ip", setLBIPPhaseName)
}

func TestShouldPatchServiceLBIP_AnnotationVariations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		annotation string
		wantPatch  bool
	}{
		{name: "empty annotation value", annotation: "", wantPatch: false},
		{name: "annotation true", annotation: "true", wantPatch: true},
		{name: "annotation True uppercase", annotation: "True", wantPatch: false},
		{name: "annotation 1", annotation: "1", wantPatch: false},
		{name: "annotation yes", annotation: "yes", wantPatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			service := &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{setLBIPAnnotation: tt.annotation},
				},
			}

			req.Equal(tt.wantPatch, shouldPatchServiceLBIP(service, []string{testOutboundIP}))
		})
	}
}

func TestRunSetLBIP_ContextCancellation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	eg := &errgroup.Group{}

	sOpts := toUpdateServiceOptions()
	data := &setLBIPPhaseData{
		ctx:      ctx,
		errGroup: eg,
		host:     createMockHostWithOutboundIP(t, ""),
		cluster:  &v1alpha1.IkniteCluster{Spec: v1alpha1.IkniteClusterSpec{Ip: net.ParseIP(testOutboundIP)}},
		getter:   createGetter(t, sOpts),
	}

	// Should return nil since Ip is nil
	req.NoError(runSetLBIP(data))
}

func TestShouldPatchServiceLBIP_NilAnnotations(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		ObjectMeta: metav1.ObjectMeta{Annotations: nil},
	}

	req.False(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestSetLBIPPatchGeneration(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}

	// Test that shouldPatchServiceLBIP correctly identifies services
	service.Annotations = map[string]string{setLBIPAnnotation: "true"}
	req.True(shouldPatchServiceLBIP(service, []string{testOutboundIP}))

	service.Annotations = map[string]string{setLBIPAnnotation: "false"}
	req.False(shouldPatchServiceLBIP(service, []string{testOutboundIP}))
}

func TestWatchSetLBIPServices(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	logger := testutil.TestLogger(t)
	mockServiceInterface := mockV1.NewMockServiceInterface(t)
	mockCoreV1Interface := mockV1.NewMockCoreV1Interface(t)
	mockCoreV1Interface.EXPECT().Services(metav1.NamespaceAll).Return(mockServiceInterface).Once()
	mockCoreV1Interface.EXPECT().Services("argocd").Return(mockServiceInterface).Once()
	fakeWatcher := watch.NewFake()
	var updatedService *corev1.Service
	mockServiceInterface.EXPECT().Watch(mock.Anything, mock.Anything).Return(fakeWatcher, nil).Once()
	updateChannel := make(chan struct{}, 1) // Buffered channel to avoid blocking
	mockServiceInterface.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, service *corev1.Service, _ metav1.UpdateOptions) (*corev1.Service, error) {
			updatedService = service
			updateChannel <- struct{}{} // Signal that the update was called
			return service, nil
		}).Once()

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "argocd-gateway",
			Namespace:   "argocd",
			Annotations: map[string]string{setLBIPAnnotation: "true"},
		},
	}

	var eg errgroup.Group
	eg.Go(func() error {
		return watchSetLBIPServices(t.Context(), mockCoreV1Interface, logger, testOutboundIP)
	})

	fakeWatcher.Add(service)
	<-updateChannel // Wait for the update to be called
	req.NotNil(updatedService)
	req.Len(updatedService.Status.LoadBalancer.Ingress, 1)
	fakeWatcher.Modify(updatedService)               // IPs as expected, should not trigger another update
	fakeWatcher.Delete(service)                      // Clean up by simulating deletion of the service
	service.Annotations[setLBIPAnnotation] = "false" // Change annotation to avoid patch
	fakeWatcher.Add(service)                         // Should not patch (Once still valid)
	fakeWatcher.Stop()                               // Stop the watcher to end the test

	err := eg.Wait()
	req.NoError(err)
}

// Same test as before, but with update error because the service has been deleted between the watch event and the patch
// attempt. This tests that we handle NotFound errors gracefully.
func TestWatchSetLBIPServices_ServiceDeletedBeforePatch(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	mockServiceInterface := mockV1.NewMockServiceInterface(t)
	mockCoreV1Interface := mockV1.NewMockCoreV1Interface(t)
	mockCoreV1Interface.EXPECT().Services(metav1.NamespaceAll).Return(mockServiceInterface).Once()
	mockCoreV1Interface.EXPECT().Services("argocd").Return(mockServiceInterface).Once()
	fakeWatcher := watch.NewFake()
	mockServiceInterface.EXPECT().Watch(mock.Anything, mock.Anything).Return(fakeWatcher, nil).Once()
	mockServiceInterface.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, apiErrors.NewNotFound(corev1.Resource("services"), "argocd-gateway")).Once()

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "argocd-gateway",
			Namespace:   "argocd",
			Annotations: map[string]string{setLBIPAnnotation: "true"},
		},
	}

	logger, hook := testutil.TestLoggerWithHook(t) // Create a test logger and hook to capture logs

	var eg errgroup.Group
	eg.Go(func() error {
		return watchSetLBIPServices(t.Context(), mockCoreV1Interface, logger, testOutboundIP)
	})

	fakeWatcher.Add(service)
	fakeWatcher.Stop() // Stop the watcher to end the test

	err := eg.Wait()
	req.NoError(err) // Should not return an error even though the update failed with NotFound
	req.Equal(slog.LevelWarn, hook.LastEntry().Level)
	req.Equal("Service not found when patching LB IP, it may have been deleted", hook.LastEntry().Message)
}

// Same test as before, but other kind of error when patching, to verify that we log an error but keep watching for
// other events.
func TestWatchSetLBIPServices_PatchError(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	mockServiceInterface := mockV1.NewMockServiceInterface(t)
	mockCoreV1Interface := mockV1.NewMockCoreV1Interface(t)
	mockCoreV1Interface.EXPECT().Services(metav1.NamespaceAll).Return(mockServiceInterface).Once()
	mockCoreV1Interface.EXPECT().Services("argocd").Return(mockServiceInterface).Once()
	fakeWatcher := watch.NewFake()
	mockServiceInterface.EXPECT().Watch(mock.Anything, mock.Anything).Return(fakeWatcher, nil).Once()
	mockServiceInterface.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("mock update error")).Once()

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "argocd-gateway",
			Namespace:   "argocd",
			Annotations: map[string]string{setLBIPAnnotation: "true"},
		},
	}

	logger, hook := testutil.TestLoggerWithHook(t) // Create a test logger and hook to capture logs
	var eg errgroup.Group
	eg.Go(func() error {
		return watchSetLBIPServices(t.Context(), mockCoreV1Interface, logger, testOutboundIP)
	})

	fakeWatcher.Add(service)
	fakeWatcher.Stop() // Stop the watcher to end the test

	err := eg.Wait()
	req.NoError(err) // Should not return an error even though the update failed
	req.Equal(slog.LevelError, hook.LastEntry().Level)
	req.Equal("Failed to patch LoadBalancer service", hook.LastEntry().Message)
}

func TestWatchSetLBIPServices_WatchError(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	mockServiceInterface := mockV1.NewMockServiceInterface(t)
	mockCoreV1Interface := mockV1.NewMockCoreV1Interface(t)
	mockCoreV1Interface.EXPECT().Services(metav1.NamespaceAll).Return(mockServiceInterface).Once()
	mockServiceInterface.EXPECT().Watch(mock.Anything, mock.Anything).Return(nil, errors.New("mock watch error")).Once()

	var eg errgroup.Group
	logger := testutil.TestLogger(t)
	eg.Go(func() error {
		return watchSetLBIPServices(t.Context(), mockCoreV1Interface, logger, testOutboundIP)
	})

	err := eg.Wait()
	req.Error(err)
}

func TestWatchSetLBIPServices_ChangeOneIpAddress(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	mockServiceInterface := mockV1.NewMockServiceInterface(t)
	mockCoreV1Interface := mockV1.NewMockCoreV1Interface(t)
	mockCoreV1Interface.EXPECT().Services(metav1.NamespaceAll).Return(mockServiceInterface).Once()
	mockCoreV1Interface.EXPECT().Services("argocd").Return(mockServiceInterface).Twice()
	fakeWatcher := watch.NewFake()
	var updatedService *corev1.Service
	mockServiceInterface.EXPECT().Watch(mock.Anything, mock.Anything).Return(fakeWatcher, nil).Once()
	updateChannel := make(chan struct{}, 1) // Buffered channel to avoid blocking
	mockServiceInterface.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, service *corev1.Service, _ metav1.UpdateOptions) (*corev1.Service, error) {
			updatedService = service
			updateChannel <- struct{}{} // Signal that the update was called
			return service, nil
		}).Twice()

	service := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "argocd-gateway",
			Namespace:   "argocd",
			Annotations: map[string]string{setLBIPAnnotation: "true"},
		},
	}

	var eg errgroup.Group
	logger := testutil.TestLogger(t)
	eg.Go(func() error {
		return watchSetLBIPServices(t.Context(), mockCoreV1Interface, logger, testOutboundIP, testClusterIP)
	})

	fakeWatcher.Add(service)
	<-updateChannel // Wait for the update to be called
	req.NotNil(updatedService)
	req.Len(updatedService.Status.LoadBalancer.Ingress, 2)
	otherService := updatedService.DeepCopy()
	otherService.Status.LoadBalancer.Ingress[0].IP = "192.168.1.19" // Simulate change of one IP address

	fakeWatcher.Modify(otherService) // Should trigger another update since one IP address changed
	<-updateChannel                  // Wait for the second update to be called
	req.NotNil(updatedService)
	req.Len(updatedService.Status.LoadBalancer.Ingress, 2)
	req.Contains(updatedService.Status.LoadBalancer.Ingress[0].IP, testOutboundIP)
	req.Contains(updatedService.Status.LoadBalancer.Ingress[1].IP, testClusterIP)
	fakeWatcher.Error(&apiErrors.NewInternalError(errors.New("internal error")).ErrStatus)
	fakeWatcher.Stop() // Stop the watcher to end the test

	err := eg.Wait()
	req.Error(err)
	req.Contains(err.Error(), "watch error: Internal error occurred: internal error")
}
