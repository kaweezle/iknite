// cSpell: words kstatus apimachinery stretchr
package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kaweezle/iknite/pkg/k8s"
)

func TestRESTClientGetter_BasicHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	r := &k8s.RESTClientGetter{}
	_, err := r.ToRESTConfig()
	req.Error(err)
	req.Contains(err.Error(), "client configuration is not set")

	loader := r.ToRawKubeConfigLoader()
	req.NotNil(loader)
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
		"metadata": map[string]any{
			"name": "demo",
		},
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

	nonAppKind := schema.GroupKind{Group: "apps", Kind: "Deployment"}
	defaultViewer, err := k8s.StatusViewerFor(nonAppKind)
	req.NoError(err)
	req.NotNil(defaultViewer)
}
