// cSpell: words kstatus apimachinery
package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockEngine "github.com/kaweezle/iknite/mocks/sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestNewRESTClientGetterFromKubeconfig(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := testutil.NewDummyUserHost()

	_, err := NewClientFromKubeconfig(fs, "/tmp/nonexistent-kubeconfig", testutil.TestLogger(t))
	req.Error(err)
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

func TestRESTClientGetterKStatusErrorPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	restConfig := testutil.CreateTestAPIServer(
		t,
		testutil.ContentPatchHandler("with_resources", &testutil.TestServerOptions{}),
	)

	getter := &Client{restConfig: restConfig, mapper: testutil.NewRESTMapper()}
	mapper, err := getter.ToRESTMapper()
	req.NoError(err)
	req.NotNil(mapper)
	logger := testutil.TestLogger(t)
	types, err := ValidateResourceTypes(mapper, []string{"deployments"}, logger)
	req.NoError(err)
	req.Contains(types, "deployments")

	infos, err := ResourceInfosForNamespace(getter, "default", []string{"deployments"})
	req.NoError(err)
	req.Len(infos, 1)

	states, err := WorkloadStatesForNamespace(getter, "default", []string{"deployments"})
	req.NoError(err)
	req.Len(states, 1)
	req.True(states[0].Ok)
}

func TestApplicationStatusReader_ReadStatusForObject(t *testing.T) {
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

func TestApplicationStatusReader_ReadStatus(t *testing.T) {
	t.Parallel()
	reader := &ApplicationStatusReader{}
	identifier := object.ObjMetadata{
		Name:      "demo",
		Namespace: "argocd",
		GroupKind: ApplicationSchemaGroupVersionKind.GroupKind(),
	}

	testCases := []struct {
		name            string
		obj             *unstructured.Unstructured
		getErr          error
		wantStatus      kstatus.Status
		wantMsgContains string
		wantErrContains []string
	}{
		{
			name: "ready application",
			obj: &unstructured.Unstructured{Object: map[string]any{
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
			}},
			wantStatus:      kstatus.CurrentStatus,
			wantMsgContains: "healthy and synced",
		},
		{
			name: "non ready application",
			obj: &unstructured.Unstructured{Object: map[string]any{
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
			}},
			wantStatus:      kstatus.InProgressStatus,
			wantMsgContains: "OutOfSync",
		},
		{
			name:            "object retrieval error",
			getErr:          errors.New("boom"),
			wantErrContains: []string{"while getting resource", "boom"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			m := mockEngine.NewMockClusterReader(t)
			m.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *unstructured.Unstructured) error {
					if testCase.getErr != nil {
						return testCase.getErr
					}
					*obj = *testCase.obj
					return nil
				},
			)

			status, err := reader.ReadStatus(context.Background(), m, identifier)
			if len(testCase.wantErrContains) > 0 {
				req.Error(err)
				req.Nil(status)
				for _, wantErrContains := range testCase.wantErrContains {
					req.ErrorContains(err, wantErrContains)
				}
				return
			}

			req.NoError(err)
			req.Equal(testCase.wantStatus, status.Status)
			req.Contains(status.Message, testCase.wantMsgContains)
		})
	}
}
