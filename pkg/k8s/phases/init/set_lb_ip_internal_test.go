// cSpell: words clientset corev metav apimachinery errgroup genericclioptions lbip testutil errchkjson sirupsen
package init

import (
	"context"
	"embed"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	mockGenericCLI "github.com/kaweezle/iknite/mocks/k8s.io/cli-runtime/pkg/genericclioptions"
	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const (
	testOutboundIP = "10.196.248.109"
	//nolint:lll // JSON string is long and difficult to break into multiple lines without losing readability
	serviceEvent = `{"type":"ADDED","object":{"kind":"Service","apiVersion":"v1","metadata":{"name":"argocd-gateway","namespace":"argocd","uid":"53bb5ca7-e030-41c0-9dd2-47d5acb43f6c","resourceVersion":"1065","creationTimestamp":"2026-05-10T16:14:31Z","labels":{"app.kubernetes.io/instance":"argocd-gateway","app.kubernetes.io/managed-by":"kgateway","app.kubernetes.io/name":"argocd-gateway","app.kubernetes.io/version":"v2.2.2"},"annotations":{"config.iknite.app/outbound-ip":"true"}},"spec":{"ports":[{"name":"listener-80","protocol":"TCP","port":80,"targetPort":80,"nodePort":30818},{"name":"listener-443","protocol":"TCP","port":443,"targetPort":443,"nodePort":32243}],"selector":{"app.kubernetes.io/instance":"argocd-gateway","app.kubernetes.io/name":"argocd-gateway","gateway.networking.k8s.io/gateway-name":"argocd-gateway"},"type":"LoadBalancer","sessionAffinity":"None","externalTrafficPolicy":"Cluster","ipFamilies":["IPv4"],"ipFamilyPolicy":"SingleStack","allocateLoadBalancerNodePorts":true,"internalTrafficPolicy":"Cluster"}}}`
)

//nolint:unparam // For future test cases we may want to specify different IPs
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
	getter   genericclioptions.RESTClientGetter
	errGroup *errgroup.Group
}

var _ setLBIPData = (*setLBIPPhaseData)(nil)

func (d *setLBIPPhaseData) Host() host.Host {
	return d.host
}

func (d *setLBIPPhaseData) Context() context.Context {
	return d.ctx
}

func (d *setLBIPPhaseData) RESTClientGetter() (genericclioptions.RESTClientGetter, error) {
	if d.getter == nil {
		return nil, errors.New("no getter available")
	}
	return d.getter, nil
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
		getter:   createGetter(t, sOpts),
	}

	req.NoError(runSetLBIP(data))
	req.NoError(data.errGroup.Wait())
	req.Len(sOpts.Requests, 2)
	writeLog := sOpts.Requests[1]
	req.Equal(http.MethodPut, writeLog.Method)
	req.Equal("/api/v1/namespaces/argocd/services/argocd-gateway/status", writeLog.Path)
}

func TestRunSetLBIP_FailsOnRESTClientGetterError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	data := &setLBIPPhaseData{
		ctx:      t.Context(),
		getter:   nil,
		errGroup: &errgroup.Group{},
		host:     createMockHostWithOutboundIP(t, ""),
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

	req.True(shouldPatchServiceLBIP(service, ""))
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

	req.False(shouldPatchServiceLBIP(service, testOutboundIP))
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

	req.False(shouldPatchServiceLBIP(service, testOutboundIP))
}

func TestShouldPatchServiceLBIP_IgnoresMissingAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	service := &corev1.Service{
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		ObjectMeta: metav1.ObjectMeta{},
	}

	req.False(shouldPatchServiceLBIP(service, testOutboundIP))
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

			req.Equal(tt.wantPatch, shouldPatchServiceLBIP(service, testOutboundIP))
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

	req.False(shouldPatchServiceLBIP(service, testOutboundIP))
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
	req.True(shouldPatchServiceLBIP(service, testOutboundIP))

	service.Annotations = map[string]string{setLBIPAnnotation: "false"}
	req.False(shouldPatchServiceLBIP(service, testOutboundIP))
}
