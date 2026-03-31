package k8s

// cSpell: words kstatus clientcmd

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/tools/clientcmd"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

// NewRESTClientGetterFromKubeconfig creates a RESTClientGetter using the default kubeconfig
// loading rules. If kubeconfigPath is non-empty it is used directly; otherwise KUBECONFIG
// env var and ~/.kube/config are tried in turn, with a final fall-back to in-cluster config.
func NewRESTClientGetterFromKubeconfig(kubeconfigPath string) *RESTClientGetter {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	return &RESTClientGetter{
		clientconfig: clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides),
	}
}

// WorkloadStatesForNamespaces returns the readiness state of deployments, statefulsets, and
// daemonsets in the given namespaces using kstatus to evaluate each resource.
// If namespaces is empty all namespaces are considered.
func (client *RESTClientGetter) WorkloadStatesForNamespaces(namespaces []string) ([]*v1alpha1.WorkloadState, error) {
	const resourceTypes = "deployments,statefulsets,daemonsets"

	r := resource.NewBuilder(client).
		Unstructured().
		AllNamespaces(true).
		ResourceTypeOrNameArgs(true, resourceTypes).
		ContinueOnError().
		Flatten().
		Do()

	infos, err := r.Infos()
	if err != nil {
		return nil, fmt.Errorf("failed to get resource infos: %w", err)
	}

	// Build a set for O(1) namespace look-ups.
	nsSet := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		nsSet[ns] = struct{}{}
	}

	result := make([]*v1alpha1.WorkloadState, 0, len(infos))
	for _, info := range infos {
		if len(namespaces) > 0 {
			if _, ok := nsSet[info.Namespace]; !ok {
				continue
			}
		}

		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s to unstructured: %w", info.ObjectName(), err)
		}

		obj := &unstructured.Unstructured{Object: u}
		res, err := kstatus.Compute(obj)
		if err != nil {
			result = append(result, &v1alpha1.WorkloadState{
				Namespace: info.Namespace,
				Name:      info.ObjectName(),
				Ok:        false,
				Message:   fmt.Sprintf("failed to compute status: %v", err),
			})
			continue
		}

		result = append(result, &v1alpha1.WorkloadState{
			Namespace: info.Namespace,
			Name:      info.ObjectName(),
			Ok:        res.Status == kstatus.CurrentStatus,
			Message:   res.Message,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].String() < result[j].String()
	})
	return result, nil
}
