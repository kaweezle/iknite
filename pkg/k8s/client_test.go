// cSpell: words kstatus apimachinery wrapcheck testrestmapper clientcmd genericclioptions sirupsen kyaml testutil
// cSpell: words serviceaccount
//
//nolint:errcheck // Tests
package k8s_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cliOptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/kaweezle/iknite/mocks/k8s.io/cli-runtime/pkg/genericclioptions"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/testutil"
)

type errorRESTMapper struct {
	meta.RESTMapper
	err error
}

func (m errorRESTMapper) RESTMapping(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	return nil, m.err
}

type asYAMLErrorResMap struct{ resmap.ResMap }

func (m asYAMLErrorResMap) AsYaml() ([]byte, error) { return nil, errors.New("yaml conversion boom") }

type removeErrorResMap struct{ resmap.ResMap }

//nolint:gocritic // interface
func (m removeErrorResMap) Remove(_ resid.ResId) error { return errors.New("remove boom") }

type jsonMarshalErrorObject struct{}

func (jsonMarshalErrorObject) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }

func (jsonMarshalErrorObject) DeepCopyObject() runtime.Object { return jsonMarshalErrorObject{} }

func resMapFromFixture(t *testing.T, fixture string) resmap.ResMap {
	t.Helper()
	content, err := os.ReadFile(fixture) //nolint:gosec // Test fixture
	require.NoError(t, err)
	return resMapFromYAML(t, string(content))
}

func sampleResMap(t *testing.T) resmap.ResMap {
	t.Helper()
	return resMapFromFixture(t, "testdata/sample_resources.yaml")
}

func resMapFromYAML(t *testing.T, yaml string) resmap.ResMap {
	t.Helper()
	factory := resmap.NewFactory(provider.NewDefaultDepProvider().GetResourceFactory())
	resources, err := factory.NewResMapFromBytes([]byte(yaml))
	require.NoError(t, err)
	return resources
}

func newUnknownWorkloadRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
	mapper.AddSpecific(
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Mystery"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"},
		meta.RESTScopeNamespace,
	)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, meta.RESTScopeNamespace)
	return mapper
}

func createWorkloadServer(t *testing.T, failPath string, includeApplications bool) cliOptions.RESTClientGetter {
	t.Helper()

	return testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewWorkloadRESTMapper(includeApplications),
		testutil.ContentPatchHandler(
			"with_resources",
			&testutil.TestServerOptions{
				Overrides: map[string]testutil.HandlerOverrideFunc{failPath: testutil.FailOverrideHandler},
			},
		),
	)
}

func TestClient_BasicHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	r := &k8s.Client{}
	_, err := r.ToRESTConfig()
	req.Error(err)
	req.Contains(err.Error(), "client configuration is not set")

	loader := r.ToRawKubeConfigLoader()
	req.NotNil(loader)
}

func TestNewDefaultClient(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := host.NewMemMapFS()
	req.NoError(testutil.CreateBasicConfig(fs, kubeadmConstants.GetAdminKubeConfigPath(), ""))
	client, err := k8s.NewDefaultClient(fs)
	req.NoError(err)
	req.NotNil(client)
	restConfig, err := client.ToRESTConfig()
	req.NoError(err)
	req.Equal("https://127.0.0.1:6443", restConfig.Host)
	req.True(k8s.IsConfigServerAddress(client, "127.0.0.1"))
}

func TestResourceInfosFromResMap(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	restConfig := testutil.CreateTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("Received request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
		switch r.Method {
		case http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logrus.Errorf("Failed to read request body: %v", err)
			} else {
				logrus.Infof("Request body: %s", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		default:
			logrus.Warnf("Unexpected request: %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	fs := host.NewMemMapFS()
	testutil.WriteRestConfigToFile(
		t,
		restConfig,
		fs,
		kubeadmConstants.GetAdminKubeConfigPath(),
		"iknite-test",
		"static",
	)
	client, err := k8s.NewDefaultClient(fs)
	req.NoError(err)

	resources := sampleResMap(t)

	_, err = k8s.ResourceInfosFromResMap(client, resources)
	req.NoError(err)

	mapper := testutil.NewRESTMapper()
	mGetter := genericclioptions.NewMockRESTClientGetter(t)
	mGetter.EXPECT().ToRESTMapper().Return(mapper, nil).Maybe()
	mGetter.EXPECT().ToRESTConfig().Return(restConfig, nil).Maybe()

	infos, err := k8s.ResourceInfosFromResMap(mGetter, resources)
	req.NoError(err)
	req.Len(infos, 5)

	err = k8s.ApplyResourceInfosServerSide(infos)
	req.NoError(err)

	ids, err := k8s.ApplyResMapWithServerSideApply(mGetter, resources)
	req.NoError(err)
	req.Len(ids, 5)
}

func TestStatusViewerForAndApplicationStatusViewer(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	viewer, err := k8s.StatusViewerFor(k8s.ApplicationSchemaGroupVersionKind.GroupKind())
	req.NoError(err)
	req.IsType(&k8s.ApplicationStatusViewer{}, viewer)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "demo"},
		"status": map[string]any{
			"sync":   map[string]any{"status": "Synced"},
			"health": map[string]any{"status": "Healthy", "message": "ok"},
		},
	}}

	msg, ready, err := viewer.Status(obj, 0)
	req.NoError(err)
	req.True(ready)
	req.Contains(msg, "demo")
	req.Contains(msg, "Synced")

	_, ready, err = viewer.Status(&unstructured.Unstructured{Object: map[string]any{"kind": "Application"}}, 0)
	req.NoError(err)
	req.False(ready)

	_, _, err = viewer.Status(&unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   "invalid",
	}}, 0)
	req.Error(err)

	defaultViewer, err := k8s.StatusViewerFor(schema.GroupKind{Group: "apps", Kind: "Deployment"})
	req.NoError(err)
	req.NotNil(defaultViewer)
}

func TestClient_ErrorAndDiscoveryPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	_, err := k8s.NewClientFromFile(fs, "/tmp/missing-kubeconfig")
	req.Error(err)
	req.NoError(fs.WriteFile("/tmp/invalid-kubeconfig", []byte("not-a-kubeconfig"), 0o644))
	_, err = k8s.NewClientFromFile(fs, "/tmp/invalid-kubeconfig")
	req.Error(err)

	r := k8s.NewClientFromConfig(&api.Config{})
	_, err = r.ToRESTConfig()
	req.Error(err)
	_, err = r.ToDiscoveryClient()
	req.Error(err)
	_, err = k8s.ClientSet(r)
	req.Error(err)
	_, err = r.ToRESTMapper()
	req.Error(err)
	_, err = k8s.RESTClient(r)
	req.Error(err)

	restConfig := testutil.CreateTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
		case "/apis":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	valid := k8s.NewClientFromRestConfig(restConfig)
	_, err = valid.ToDiscoveryClient()
	req.NoError(err)
	_, err = k8s.ClientSet(valid)
	req.NoError(err)
	_, err = k8s.RESTClient(valid)
	req.NoError(err)

	invalid := k8s.NewClientFromRestConfig(&rest.Config{Host: "://bad-host"})
	_, err = invalid.ToDiscoveryClient()
	req.Error(err)
	_, err = k8s.ClientSet(invalid)
	req.Error(err)
	_, err = k8s.RESTClient(invalid)
	req.Error(err)
}

func TestApplyResourceInfosServerSide_ErrorPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	client := testutil.CreateClientGetterWithTestServer(t, testutil.NewRESTMapper(),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

	infos, err := k8s.ResourceInfosFromResMap(client, sampleResMap(t))
	req.NoError(err)

	err = k8s.ApplyResourceInfosServerSide(infos)
	req.Error(err)

	err = k8s.ApplyResourceInfosServerSide([]*resource.Info{{
		Name:      "marshal-error",
		Namespace: "default",
		Object: &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "marshal-error", "namespace": "default"},
			"bad":        make(chan int),
		}},
	}})
	req.Error(err)
	req.Contains(err.Error(), "failed to marshal resource")

	err = k8s.ApplyResourceInfosServerSide(
		[]*resource.Info{{Name: "convert-error", Namespace: "default", Object: jsonMarshalErrorObject{}}},
	)
	req.Error(err)
	req.Contains(err.Error(), "failed to convert resource")
}

func TestApplyResMapWithServerSideApply_Branches(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	getter := genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(nil, errors.New("mapper boom"))
	_, err := k8s.ApplyResMapWithServerSideApply(getter, sampleResMap(t))
	req.Error(err)
	req.Contains(err.Error(), "failed to build cluster resource infos")

	_, err = k8s.ResourceInfosFromResMap(getter, asYAMLErrorResMap{})
	req.Error(err)

	realGetter := testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewRESTMapper(),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	)
	_, err = k8s.ApplyResMapWithServerSideApply(
		realGetter,
		resMapFromYAML(
			t,
			"apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: cr\nrules: []\n",
		),
	)
	req.Error(err)
	req.Contains(err.Error(), "failed to apply cluster resources")

	getter = genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(nil, errors.New("mapper boom")).Maybe()
	_, err = k8s.ApplyResMapWithServerSideApply(
		getter,
		resMapFromYAML(
			t,
			"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\ndata:\n  k: v\n",
		),
	)
	req.Error(err)
	req.Contains(err.Error(), "failed to build resource infos")

	nsPatchFail := testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewRESTMapper(),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	)

	nsMap := resMapFromYAML(
		t,
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm2\n  namespace: default\ndata:\n  k: v\n",
	)
	_, err = k8s.ApplyResMapWithServerSideApply(nsPatchFail, nsMap)
	req.Error(err)
	req.Contains(err.Error(), "failed to apply resources")

	removeServer := testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewRESTMapper(),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				logrus.Errorf("Failed to read request body: %v", readErr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		},
	)
	_, err = k8s.ApplyResMapWithServerSideApply(
		removeServer,
		removeErrorResMap{
			ResMap: resMapFromYAML(
				t,
				"apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: remove-err\nrules: []\n",
			),
		},
	)
	req.Error(err)
	req.Contains(err.Error(), "failed to remove cluster-scoped resource")
}

func TestHasApplicationsAndAllWorkloadStates(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	has, err := k8s.HasApplications(testutil.NewWorkloadRESTMapper(false))
	req.NoError(err)
	req.False(has)
	has, err = k8s.HasApplications(testutil.NewWorkloadRESTMapper(true))
	req.NoError(err)
	req.True(has)
	_, err = k8s.HasApplications(
		errorRESTMapper{RESTMapper: testutil.NewWorkloadRESTMapper(true), err: errors.New("mapping boom")},
	)
	req.Error(err)

	getter := genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(nil, errors.New("mapper unavailable")).Once()
	_, err = k8s.AllWorkloadStates(getter)
	req.Error(err)

	getter = genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().
		ToRESTMapper().
		Return(errorRESTMapper{RESTMapper: testutil.NewWorkloadRESTMapper(true), err: errors.New("mapping boom")}, nil).
		Once()
	_, err = k8s.AllWorkloadStates(getter)
	req.Error(err)

	failServer := createWorkloadServer(t, "/apis/apps/v1/deployments", false)
	_, err = k8s.AllWorkloadStates(failServer)
	req.Error(err)

	readyServer := createWorkloadServer(t, "", true)
	states, err := k8s.AllWorkloadStates(readyServer)
	req.NoError(err)
	req.Len(states, 4)

	unknownServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(
			[]byte(
				//nolint:lll // Literal
				`{"kind":"MysteryList","apiVersion":"apps/v1","items":[{"apiVersion":"apps/v1","kind":"Mystery","metadata":{"name":"mystery","namespace":"default"}}]}`,
			),
		)
	}))
	defer unknownServer.Close()
	getter = genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(newUnknownWorkloadRESTMapper(), nil).Maybe()
	getter.EXPECT().ToRESTConfig().Return(&rest.Config{Host: unknownServer.URL}, nil).Maybe()
	_, err = k8s.AllWorkloadStates(getter)
	req.Error(err)

	statusErrServerGetter := testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewRESTMapper(),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "daemonsets") {
				_, _ = w.Write(
					[]byte(
						//nolint:lll // Literal
						`{"kind":"DaemonSetList","apiVersion":"apps/v1","items":[{"apiVersion":"apps/v1","kind":"DaemonSet","metadata":{"name":"daemon-bad","namespace":"default","generation":1},"spec":{"updateStrategy":{"type":"OnDelete"}},"status":{"observedGeneration":1}}]}`,
					),
				)
				return
			}
			if strings.Contains(r.URL.Path, "statefulsets") {
				_, _ = w.Write([]byte(`{"kind":"StatefulSetList","apiVersion":"apps/v1","items":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"kind":"DeploymentList","apiVersion":"apps/v1","items":[]}`))
		},
	)

	_, err = k8s.AllWorkloadStates(statusErrServerGetter)
	req.Error(err)
	req.Contains(err.Error(), "failed to get workload status")
}

func TestWorkloadsReadyConditionWithContextFunc(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	getter := genericclioptions.NewMockRESTClientGetter(t)
	getter.EXPECT().ToRESTMapper().Return(nil, errors.New("mapper unavailable")).Once()
	condition := k8s.WorkloadsReadyConditionWithContextFunc(getter, nil)
	ready, err := condition(t.Context())
	req.Error(err)
	req.False(ready)

	server := createWorkloadServer(t, "", true)
	iterations := make([]int, 0, 2)
	okIterations := make([]int, 0, 2)
	condition = k8s.WorkloadsReadyConditionWithContextFunc(server, func(allReady bool, total int,
		readyStates []*v1alpha1.WorkloadState, unready []*v1alpha1.WorkloadState, iteration, okIteration int,
	) bool {
		req.True(allReady)
		req.Equal(4, total)
		req.Len(readyStates, 4)
		req.Empty(unready)
		iterations = append(iterations, iteration)
		okIterations = append(okIterations, okIteration)
		return allReady
	})

	ready, err = condition(t.Context())
	req.NoError(err)
	req.True(ready)
	ready, err = condition(t.Context())
	req.NoError(err)
	req.True(ready)

	req.Equal([]int{0, 1}, iterations)
	req.Equal([]int{1, 2}, okIterations)

	unreadyServerGetter := testutil.CreateClientGetterWithTestServer(
		t,
		testutil.NewWorkloadRESTMapper(false),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "deployments") {
				_, _ = w.Write(
					[]byte(
						//nolint:lll // Literal
						`{"kind":"DeploymentList","apiVersion":"apps/v1","items":[{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"deploy-unready","namespace":"default","generation":1},"spec":{"replicas":1,"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxUnavailable":0,"maxSurge":1}}},"status":{"observedGeneration":1,"replicas":1,"updatedReplicas":0,"availableReplicas":0}}]}`,
					),
				)
				return
			}
			if strings.Contains(r.URL.Path, "statefulsets") {
				_, _ = w.Write([]byte(`{"kind":"StatefulSetList","apiVersion":"apps/v1","items":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"kind":"DaemonSetList","apiVersion":"apps/v1","items":[]}`))
		},
	)
	condition = k8s.WorkloadsReadyConditionWithContextFunc(unreadyServerGetter, nil)
	ready, err = condition(t.Context())
	req.NoError(err)
	req.False(ready)
}

const (
	validCA = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIIDvrQMUvfbOgwDQYJKoZIhvcNAQELBQAwFTETMBEGA1UE
AxMKa3ViZXJuZXRlczAeFw0yNjA0MTYxMzA0MzZaFw0zNjA0MTMxMzA5MzZaMBUx
EzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQDxjLdC8Bwy5mitG0nvEVH1qdOkEXwHEhX05IU61I9LbqTjGfeYm8BOe+4I
5bYE0BkLpuQvYAkMX8TtUFfIQmYy1cepP68RHPNOUgjOjw75zAecY5O9zFWOUEsZ
wMOn8bGeEaKBIeeE73v7B4RDZuDWI0gLuCErAX2h7hfc+6H8yDiEg5XfF7hw8TjJ
E3oDgoKkD0dfBqHSvlV+mie2mX5LCBaWapupM0837ZvnQMYHwPdTun4+LHKtqczh
/LXvDJDLlkx6BaodwUuzy1sQXqtDQTqClpTUx1lvL8gGwfJL3wb5t9zNiX4xMu9U
rBURJJqb5WczCUGZMGRZPYxFRei/AgMBAAGjWTBXMA4GA1UdDwEB/wQEAwICpDAP
BgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRvGLRJ3zGrtLd++3QT1+RAwfG12TAV
BgNVHREEDjAMggprdWJlcm5ldGVzMA0GCSqGSIb3DQEBCwUAA4IBAQDrJkX8Cbgy
Vo57PI4hVtNmUw0wBlGITInKQqj6QDe7I8bFdq8VIC6J2+vH5Mc16ueJ/N7k1Dsb
ODmRX3J0pTYCyCxvsJG/dr7Y8V3a/+EDIgsYhnpWSD4XsQVYNK+EKh1O+tP14FVv
0nD8GimvmjQPigi30YgEOvD0VvQRuBvp4RR6LK/WxnuuLDVdeyKy/hTIi27mAWBE
5q8+L3yvWh9VNZYtoNxpzjAvJ6i8ASHNEfC4OWJnpsifYO8zzZ7QIl1tOtwIchWz
kLKx5H11WJWssDV8k6Z1yNzMOxF3iQNsbFifY54q3+Ji55PZqjsC4OaveS/gngc9
o+83dWx3ngrT
-----END CERTIFICATE-----
`
	serviceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount"
	tokenFile         = serviceAccountDir + "/token"
	caFile            = serviceAccountDir + "/ca.crt"
)

func TestInClusterConfig_MissingServiceHost(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	fs := host.NewMemMapFS()
	config, err := k8s.InClusterConfig(fs)
	req.ErrorIs(err, rest.ErrNotInCluster)
	req.Nil(config)
}

func TestInClusterConfig_MissingServicePort(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	fs := host.NewMemMapFS()
	config, err := k8s.InClusterConfig(fs)
	req.ErrorIs(err, rest.ErrNotInCluster)
	req.Nil(config)
}

func TestInClusterConfig_NoBothServiceHost(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	fs := host.NewMemMapFS()
	config, err := k8s.InClusterConfig(fs)
	req.ErrorIs(err, rest.ErrNotInCluster)
	req.Nil(config)
}

func TestInClusterConfig_TokenFileNotFound(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	fs := host.NewMemMapFS()
	config, err := k8s.InClusterConfig(fs)
	req.Error(err)
	req.Nil(config)
}

func TestInClusterConfig_ValidTokenWithNoCA(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	fs := host.NewMemMapFS()

	req.NoError(fs.MkdirAll(serviceAccountDir, 0o755))
	req.NoError(fs.WriteFile(tokenFile, []byte("valid-token"), 0o644))

	config, err := k8s.InClusterConfig(fs)
	req.NoError(err)
	req.NotNil(config)
	req.Equal("https://kubernetes.default.svc:6443", config.Host)
	req.Equal("valid-token", config.BearerToken)
	req.Equal(tokenFile, config.BearerTokenFile)
	// CAFile should be empty because cert.NewPool fails to load non-existent CA
	req.Empty(config.CAFile)
}

func TestInClusterConfig_ValidTokenWithInvalidCA(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	fs := host.NewMemMapFS()

	req.NoError(fs.MkdirAll(serviceAccountDir, 0o755))
	req.NoError(fs.WriteFile(tokenFile, []byte("valid-token"), 0o644))
	req.NoError(fs.WriteFile(caFile, []byte("invalid-ca"), 0o644))

	config, err := k8s.InClusterConfig(fs)
	req.NoError(err)
	req.NotNil(config)
	req.Equal("https://kubernetes.default.svc:6443", config.Host)
	req.Equal("valid-token", config.BearerToken)
	req.Equal(tokenFile, config.BearerTokenFile)
	// CAFile should be empty because cert.NewPool fails to load non-existent CA
	req.Empty(config.CAFile)
}

func TestInClusterConfig_ValidTokenWithValidCA(t *testing.T) {
	req := require.New(t)
	t.Setenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	fs := host.NewMemMapFS()

	req.NoError(fs.MkdirAll(serviceAccountDir, 0o755))
	req.NoError(fs.WriteFile(tokenFile, []byte("valid-token"), 0o644))
	req.NoError(fs.WriteFile(caFile, []byte(validCA), 0o644))

	config, err := k8s.InClusterConfig(fs)
	req.NoError(err)
	req.NotNil(config)
	req.Equal("https://kubernetes.default.svc:6443", config.Host)
	req.Equal("valid-token", config.BearerToken)
	req.Equal(tokenFile, config.BearerTokenFile)
	// CAFile should be empty because cert.NewPool fails to load non-existent CA
	req.NotEmpty(config.CAData)
}
