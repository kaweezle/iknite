package k8s

// cSpell: words kstatus clientcmd apimachinery clientconfig sirupsen

import (
	"context"
	"fmt"
	"sort"
	"strings"

	argoV1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	runTimeClient "sigs.k8s.io/controller-runtime/pkg/client"

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

// workloadStatesToSlice converts resource.Info objects to WorkloadState using kstatus.
func workloadStatesToSlice(infos []*resource.Info) ([]*v1alpha1.WorkloadState, error) {
	result := make([]*v1alpha1.WorkloadState, 0, len(infos))
	for _, info := range infos {
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

func (client *RESTClientGetter) ValidateResourceTypes(resourceTypes string) ([]string, error) {
	types := strings.Split(resourceTypes, ",")
	validTypes := make([]string, 0, len(types))
	for _, t := range types {
		has, err := client.HasResourceType(t)
		if err != nil {
			return nil, fmt.Errorf("while checking resource type %s: %w", t, err)
		}
		if has {
			validTypes = append(validTypes, t)
		} else {
			log.WithField("resourceType", t).Warn("Non existent resource type, removing...")
		}
	}
	return validTypes, nil
}

func (client *RESTClientGetter) HasResourceType(resourceType string) (bool, error) {
	discovery, err := client.ToDiscoveryClient()
	if err != nil {
		return false, fmt.Errorf("while getting discovery client: %w", err)
	}
	list, err := discovery.ServerPreferredResources()
	if err != nil {
		return false, fmt.Errorf("while getting server preferred resources: %w", err)
	}
	for _, apiResourceList := range list {
		for i := range apiResourceList.APIResources {
			if apiResourceList.APIResources[i].Name == resourceType {
				return true, nil
			}
		}
	}
	return false, nil
}

// ResourceInfosForNamespace returns resource.Info objects for the given namespace and resource types.
func (client *RESTClientGetter) ResourceInfosForNamespace(namespace, resourceTypes string) ([]*resource.Info, error) {
	r := resource.NewBuilder(client).
		Unstructured().
		NamespaceParam(namespace).
		DefaultNamespace().
		ResourceTypeOrNameArgs(true, resourceTypes).
		ContinueOnError().
		Flatten().
		Do()

	infos, err := r.Infos()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get resource types %s infos in namespace %s: %w",
			resourceTypes,
			namespace,
			err,
		)
	}
	return infos, nil
}

func infosToObjectMetadataSet(infos []*resource.Info) object.ObjMetadataSet {
	set := make(object.ObjMetadataSet, 0, len(infos))
	for _, info := range infos {
		set = append(set, object.ObjMetadata{
			Namespace: info.Namespace,
			Name:      info.Name,
			GroupKind: info.Object.GetObjectKind().GroupVersionKind().GroupKind(),
		})
	}
	return set
}

func (client *RESTClientGetter) ObjectMetadataSetForNamespace(
	namespace, resourceTypes string,
) (object.ObjMetadataSet, error) {
	infos, err := client.ResourceInfosForNamespace(namespace, resourceTypes)
	if err != nil {
		return nil, fmt.Errorf("while getting object metadata set for namespace %s: %w", namespace, err)
	}
	return infosToObjectMetadataSet(infos), nil
}

// WorkloadStatesForNamespace returns the readiness state of deployments, statefulsets, and
// daemonsets in a single namespace using kstatus to evaluate each resource.
func (client *RESTClientGetter) WorkloadStatesForNamespace(
	namespace, resourceTypes string,
) ([]*v1alpha1.WorkloadState, error) {
	infos, err := client.ResourceInfosForNamespace(namespace, resourceTypes)
	if err != nil {
		return nil, fmt.Errorf("while getting workload states for namespace %s: %w", namespace, err)
	}

	return workloadStatesToSlice(infos)
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

	filtered := make([]*resource.Info, 0, len(infos))
	for _, info := range infos {
		if len(namespaces) == 0 {
			filtered = append(filtered, info)
			continue
		}
		if _, ok := nsSet[info.Namespace]; ok {
			filtered = append(filtered, info)
		}
	}

	return workloadStatesToSlice(filtered)
}

type ApplicationStatusReader struct{}

var _ engine.StatusReader = (*ApplicationStatusReader)(nil)

const (
	applicationSyncedStatus  = "Synced"
	applicationHealthyStatus = "Healthy"
)

var applicationGVK = schema.GroupVersionKind{
	Group:   "argoproj.io",
	Version: "v1alpha1",
	Kind:    "Application",
}

func (r *ApplicationStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == applicationGVK.GroupKind()
}

func (r *ApplicationStatusReader) ReadStatus(
	ctx context.Context,
	reader engine.ClusterReader,
	identifier object.ObjMetadata,
) (*event.ResourceStatus, error) {
	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(applicationGVK)
	key := runTimeClient.ObjectKey{Namespace: identifier.Namespace, Name: identifier.Name}
	err := reader.Get(ctx, key, &obj)
	if err != nil {
		return nil, fmt.Errorf("while getting resource %s: %w", identifier, err)
	}
	app := &argoV1.Application{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), app); err != nil {
		return nil, fmt.Errorf("while converting unstructured to Application: %w", err)
	}
	status := &event.ResourceStatus{
		Status:     kstatus.UnknownStatus,
		Identifier: identifier,
		Message:    "Application status not available",
	}
	if app.Status.Sync.Status == applicationSyncedStatus && app.Status.Health.Status == applicationHealthyStatus {
		status.Status = kstatus.CurrentStatus
		status.Message = fmt.Sprintf("Application is healthy and synced on revision %s", app.Status.Sync.Revision[0:7])
	} else {
		status.Status = kstatus.InProgressStatus
		status.Message = fmt.Sprintf(
			"Application sync status: %s, health status: %s",
			app.Status.Sync.Status,
			app.Status.Health.Status,
		)
	}
	return status, nil
}

func (r *ApplicationStatusReader) ReadStatusForObject(
	_ context.Context,
	_ engine.ClusterReader,
	obj *unstructured.Unstructured,
) (*event.ResourceStatus, error) {
	identifier := object.UnstructuredToObjMetadata(obj)
	app := &argoV1.Application{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), app); err != nil {
		return nil, fmt.Errorf("while converting unstructured to Application: %w", err)
	}
	status := &event.ResourceStatus{
		Status:     kstatus.UnknownStatus,
		Identifier: identifier,
		Resource:   obj,
		Message:    "Application status not available",
	}
	if app.Status.Sync.Status == applicationSyncedStatus && app.Status.Health.Status == applicationHealthyStatus {
		status.Status = kstatus.CurrentStatus
		status.Message = fmt.Sprintf("Application is healthy and synced on revision %s", app.Status.Sync.Revision[0:7])
	} else {
		status.Status = kstatus.InProgressStatus
		status.Message = fmt.Sprintf(
			"Application sync status: %s, health status: %s",
			app.Status.Sync.Status,
			app.Status.Health.Status,
		)
	}
	return status, nil
}
