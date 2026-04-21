// cSpell: words kstatus apimachinery sirupsen
package k8s

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	certUtil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestNewRESTClientGetterFromKubeconfig(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	getter := NewClientFromKubeconfig("/tmp/nonexistent-kubeconfig")
	req.NotNil(getter)
	req.NotNil(getter.ToRawKubeConfigLoader())
}

func TestWorkloadStatesToSliceAndInfosToMetadataSet(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	obj1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "b",
			"namespace": "ns",
		},
	}}
	obj2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "a",
			"namespace": "ns",
		},
	}}

	infos := []*resource.Info{{Name: "b", Namespace: "ns", Object: obj1}, {Name: "a", Namespace: "ns", Object: obj2}}
	states, err := workloadStatesToSlice(infos)
	req.NoError(err)
	req.Len(states, 2)
	req.Contains(states[0].Name, "a")
	req.Contains(states[1].Name, "b")

	set := infosToObjectMetadataSet(infos)
	req.Len(set, 2)
	req.Equal("ns", set[0].Namespace)
}

// cSpell: disable
//
//nolint:lll // Raw response
const apisResponse = `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"apiregistration.k8s.io","versions":[{"groupVersion":"apiregistration.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apiregistration.k8s.io/v1","version":"v1"}},{"name":"apps","versions":[{"groupVersion":"apps/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apps/v1","version":"v1"}},{"name":"events.k8s.io","versions":[{"groupVersion":"events.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"events.k8s.io/v1","version":"v1"}},{"name":"authentication.k8s.io","versions":[{"groupVersion":"authentication.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"authentication.k8s.io/v1","version":"v1"}},{"name":"authorization.k8s.io","versions":[{"groupVersion":"authorization.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"authorization.k8s.io/v1","version":"v1"}},{"name":"autoscaling","versions":[{"groupVersion":"autoscaling/v2","version":"v2"},{"groupVersion":"autoscaling/v1","version":"v1"}],"preferredVersion":{"groupVersion":"autoscaling/v2","version":"v2"}},{"name":"batch","versions":[{"groupVersion":"batch/v1","version":"v1"}],"preferredVersion":{"groupVersion":"batch/v1","version":"v1"}},{"name":"certificates.k8s.io","versions":[{"groupVersion":"certificates.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"certificates.k8s.io/v1","version":"v1"}},{"name":"networking.k8s.io","versions":[{"groupVersion":"networking.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"networking.k8s.io/v1","version":"v1"}},{"name":"policy","versions":[{"groupVersion":"policy/v1","version":"v1"}],"preferredVersion":{"groupVersion":"policy/v1","version":"v1"}},{"name":"rbac.authorization.k8s.io","versions":[{"groupVersion":"rbac.authorization.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"rbac.authorization.k8s.io/v1","version":"v1"}},{"name":"storage.k8s.io","versions":[{"groupVersion":"storage.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"storage.k8s.io/v1","version":"v1"}},{"name":"admissionregistration.k8s.io","versions":[{"groupVersion":"admissionregistration.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"admissionregistration.k8s.io/v1","version":"v1"}},{"name":"apiextensions.k8s.io","versions":[{"groupVersion":"apiextensions.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apiextensions.k8s.io/v1","version":"v1"}},{"name":"scheduling.k8s.io","versions":[{"groupVersion":"scheduling.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"scheduling.k8s.io/v1","version":"v1"}},{"name":"coordination.k8s.io","versions":[{"groupVersion":"coordination.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"coordination.k8s.io/v1","version":"v1"}},{"name":"node.k8s.io","versions":[{"groupVersion":"node.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"node.k8s.io/v1","version":"v1"}},{"name":"discovery.k8s.io","versions":[{"groupVersion":"discovery.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"discovery.k8s.io/v1","version":"v1"}},{"name":"resource.k8s.io","versions":[{"groupVersion":"resource.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"resource.k8s.io/v1","version":"v1"}},{"name":"flowcontrol.apiserver.k8s.io","versions":[{"groupVersion":"flowcontrol.apiserver.k8s.io/v1","version":"v1"}],"preferredVersion":{"groupVersion":"flowcontrol.apiserver.k8s.io/v1","version":"v1"}},{"name":"gateway.networking.k8s.io","versions":[{"groupVersion":"gateway.networking.k8s.io/v1","version":"v1"},{"groupVersion":"gateway.networking.k8s.io/v1beta1","version":"v1beta1"}],"preferredVersion":{"groupVersion":"gateway.networking.k8s.io/v1","version":"v1"}},{"name":"argoproj.io","versions":[{"groupVersion":"argoproj.io/v1alpha1","version":"v1alpha1"}],"preferredVersion":{"groupVersion":"argoproj.io/v1alpha1","version":"v1alpha1"}},{"name":"gateway.kgateway.dev","versions":[{"groupVersion":"gateway.kgateway.dev/v1alpha1","version":"v1alpha1"}],"preferredVersion":{"groupVersion":"gateway.kgateway.dev/v1alpha1","version":"v1alpha1"}},{"name":"metrics.k8s.io","versions":[{"groupVersion":"metrics.k8s.io/v1beta1","version":"v1beta1"}],"preferredVersion":{"groupVersion":"metrics.k8s.io/v1beta1","version":"v1beta1"}}]} `

//
//nolint:lll // Raw response
const apiResponse = `{"kind":"APIVersions","versions":["v1"],"serverAddressByClientCIDRs":[{"clientCIDR":"0.0.0.0/0","serverAddress":"192.168.99.2:6443"}]}`

// TODO: Use the following when testing mock RESTMapper
// var defaultResourceTypes = []string{"deployments", "statefulsets", "daemonsets", "jobs", "cronjobs", "applications"}

// cSpell: enable

func TestRESTClientGetterKStatusErrorPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("Received request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(apiResponse)) //nolint:errcheck // test server, ignore error
		case r.Method == http.MethodGet && r.URL.Path == "/apis":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(apisResponse)) //nolint:errcheck // test server, ignore error
		default:
			logrus.Warnf("Unexpected request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	cert := server.Certificate()
	req.NotNil(cert)
	pem, err := certUtil.EncodeCertificates(cert)
	req.NoError(err)

	getter := &Client{restConfig: &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: pem,
		},
	}}
	mapper, err := getter.ToRESTMapper()
	req.NoError(err)
	req.NotNil(mapper)
	types, err := ValidateResourceTypes(mapper, []string{"deployments"})
	req.NoError(err)
	req.Empty(types)

	_, err = ResourceInfosForNamespace(getter, "default", []string{"deployments"})
	req.Error(err)

	_, err = WorkloadStatesForNamespace(getter, "default", []string{"deployments"})
	req.Error(err)
}

func TestApplicationStatusReader(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	reader := &ApplicationStatusReader{}
	req.True(reader.Supports(applicationGVK.GroupKind()))
	req.False(reader.Supports(schema.GroupKind{Group: "apps", Kind: "Deployment"}))

	syncedHealthy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "argocd",
		},
		"status": map[string]any{
			"sync":   map[string]any{"status": "Synced", "revision": "1234567890abcdef"},
			"health": map[string]any{"status": "Healthy"},
		},
	}}
	res, err := reader.ReadStatusForObject(context.Background(), nil, syncedHealthy)
	req.NoError(err)
	req.Equal(kstatus.CurrentStatus, res.Status)
	req.Contains(res.Message, "healthy and synced")

	inProgress := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "argocd",
		},
		"status": map[string]any{
			"sync":   map[string]any{"status": "OutOfSync", "revision": "abcdef1234567"},
			"health": map[string]any{"status": "Progressing"},
		},
	}}
	res, err = reader.ReadStatusForObject(context.Background(), nil, inProgress)
	req.NoError(err)
	req.Equal(kstatus.InProgressStatus, res.Status)
	req.Contains(res.Message, "OutOfSync")

	res, err = reader.ReadStatusForObject(
		context.Background(),
		nil,
		&unstructured.Unstructured{Object: map[string]any{"kind": "Application"}},
	)
	req.NoError(err)
	req.Equal(kstatus.InProgressStatus, res.Status)

	_ = event.ResourceStatus{}
}
