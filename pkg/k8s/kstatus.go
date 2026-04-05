package k8s

// cSpell: words kstatus clientcmd apimachinery clientconfig sirupsen

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	argoV1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	clientRest "k8s.io/client-go/rest"
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
	restConfig, err := clientRest.InClusterConfig()
	if err == nil {
		log.Info("Using in-cluster configuration")
		return &RESTClientGetter{
			clientconfig: nil, // Not needed for in-cluster config
			restConfig:   restConfig,
		}
	}
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

// ValidateResourceTypes checks if the provided resource types are valid by attempting to find their corresponding
// GroupVersionKind in the REST mapper. It returns a slice of valid resource types and an error if the
// validation process encounters an issue.
func (client *RESTClientGetter) ValidateResourceTypes(types []string) ([]string, error) {
	restMapper, err := client.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("while getting REST mapper: %w", err)
	}

	validTypes := make([]string, 0, len(types))
	noMatchError := &meta.NoResourceMatchError{}
	for _, t := range types {
		const maxAttempts = 3
		const retryDelay = 1 * time.Second

		gvr := schema.GroupVersionResource{Group: "", Version: "", Resource: t}
		_, err := restMapper.KindFor(gvr)
		for attempt := 2; err != nil && !errors.Is(err, noMatchError) && attempt <= maxAttempts; attempt++ {
			log.WithError(err).WithFields(log.Fields{
				"resourceType": t,
				"attempt":      attempt,
			}).Warn("Failed to get resource infos, retrying")
			time.Sleep(retryDelay)
			_, err = restMapper.KindFor(gvr)
		}

		if err != nil {
			if errors.Is(err, noMatchError) {
				log.WithError(err).
					WithField("resourceType", t).
					Warn("Resource type not found in REST mapper, skipping...")
				continue
			}
			return nil, fmt.Errorf("while validating resource type %s: %w", t, err)
		}
		log.WithField("resourceType", t).Info("Resource type is available in REST mapper")
		validTypes = append(validTypes, t)
	}
	return validTypes, nil
}

// ResourceInfosForNamespace returns resource.Info objects for the given namespace and resource types.
func (client *RESTClientGetter) ResourceInfosForNamespace(
	namespace string,
	resourceTypes []string,
) ([]*resource.Info, error) {
	r := resource.NewBuilder(client).
		Unstructured().
		NamespaceParam(namespace).
		DefaultNamespace().
		ResourceTypes(resourceTypes...).
		SelectAllParam(true).
		ContinueOnError().
		Flatten().
		Latest().
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
	namespace string, resourceTypes []string,
) (object.ObjMetadataSet, error) {
	const maxAttempts = 3
	const retryDelay = 2 * time.Second

	infos, err := client.ResourceInfosForNamespace(namespace, resourceTypes)
	for attempt := 2; err != nil && attempt <= maxAttempts; attempt++ {
		log.WithError(err).WithFields(log.Fields{
			"resourceTypes": resourceTypes,
			"attempt":       attempt,
			"namespace":     namespace,
		}).Warn("Failed to get resource infos, retrying")
		time.Sleep(retryDelay)
		infos, err = client.ResourceInfosForNamespace(namespace, resourceTypes)
	}
	if err != nil {
		return nil, fmt.Errorf("while getting object metadata set for namespace %s: %w", namespace, err)
	}
	return infosToObjectMetadataSet(infos), nil
}

// WorkloadStatesForNamespace returns the readiness state of deployments, statefulsets, and
// daemonsets in a single namespace using kstatus to evaluate each resource.
func (client *RESTClientGetter) WorkloadStatesForNamespace(
	namespace string, resourceTypes []string,
) ([]*v1alpha1.WorkloadState, error) {
	infos, err := client.ResourceInfosForNamespace(namespace, resourceTypes)
	if err != nil {
		return nil, fmt.Errorf("while getting workload states for namespace %s: %w", namespace, err)
	}

	return workloadStatesToSlice(infos)
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
