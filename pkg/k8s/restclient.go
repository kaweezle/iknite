package k8s

// cSpell: words clientcmd clientconfig restconfig casttype metav1 polymorphichelpers restmapper
// cSpell: disable
import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/polymorphichelpers"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
)

// cSpell: enable

type RESTClientGetter struct {
	clientconfig clientcmd.ClientConfig
}

func (config *Config) RESTClient() *RESTClientGetter {
	return &RESTClientGetter{clientcmd.NewDefaultClientConfig(api.Config(*config), nil)}
}

func (r *RESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	restConfig, err := r.clientconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}
	return restConfig, nil
}

func (r *RESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restconfig, err := r.clientconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config for discovery: %w", err)
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	return memory.NewMemCacheClient(dc), nil
}

func (r *RESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (r *RESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return r.clientconfig
}

var ApplicationSchemaGroupVersionKind = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}

type SyncStatus struct {
	Status string `json:"status" protobuf:"bytes,1,opt,name=status,casttype=SyncStatusCode"`
}

type HealthStatus struct {
	// Status holds the status code of the application or resource
	Status string `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	// Message is a human-readable informational message describing the health status
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}

type ApplicationStatus struct {
	Sync   SyncStatus   `json:"sync,omitempty" protobuf:"bytes,2,opt,name=sync"`
	Health HealthStatus `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`
}

type Application struct {
	Status            ApplicationStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
}

type ApplicationStatusViewer struct{}

func StatusViewerFor(kind schema.GroupKind) (polymorphichelpers.StatusViewer, error) {
	if kind == ApplicationSchemaGroupVersionKind.GroupKind() {
		return &ApplicationStatusViewer{}, nil
	}
	sv, err := polymorphichelpers.StatusViewerFor(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to get status viewer for %v: %w", kind, err)
	}
	return sv, nil
}

func (s *ApplicationStatusViewer) Status(obj runtime.Unstructured, revision int64) (string, bool, error) {
	application := &Application{}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), application)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert %T to %T: %w", obj, application, err)
	}

	healthStatusString := application.Status.Health.Status
	syncStatusString := application.Status.Sync.Status

	msg := fmt.Sprintf("application \"%s\" sync status: %s, health status: %s", application.Name, syncStatusString, healthStatusString)
	return msg, healthStatusString == "Healthy" && syncStatusString == "Synced", nil
}

func (client *RESTClientGetter) HasApplications() (has bool, err error) {
	var mapper meta.RESTMapper
	if mapper, err = client.ToRESTMapper(); err != nil {
		return has, err
	}

	_, err = mapper.RESTMapping(ApplicationSchemaGroupVersionKind.GroupKind(), ApplicationSchemaGroupVersionKind.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			err = nil
		} else {
			return has, err
		}
	} else {
		has = true
	}
	return has, err
}

func (client *RESTClientGetter) AllWorkloadStates() (result []*v1alpha1.WorkloadState, err error) {
	var _result []*v1alpha1.WorkloadState

	resourceTypes := "deployments,statefulsets,daemonsets"
	var hasApplications bool
	if hasApplications, err = client.HasApplications(); err != nil {
		return result, err
	}
	if hasApplications {
		resourceTypes += ",applications"
	}

	r := resource.NewBuilder(client).
		Unstructured().
		AllNamespaces(true).
		ResourceTypeOrNameArgs(true, resourceTypes).
		ContinueOnError().
		Flatten().
		Do()

	var infos []*resource.Info
	if infos, err = r.Infos(); err != nil {
		return result, fmt.Errorf("failed to get resource infos: %w", err)
	}

	for _, info := range infos {
		var u map[string]any

		if u, err = runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object); err != nil {
			return result, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}

		var v polymorphichelpers.StatusViewer
		if v, err = /* polymorphichelpers. */ StatusViewerFor(info.Object.GetObjectKind().GroupVersionKind().GroupKind()); err != nil {
			return result, fmt.Errorf("failed to get status viewer: %w", err)
		}

		var msg string
		var ok bool
		if msg, ok, err = v.Status(&unstructured.Unstructured{Object: u}, 0); err != nil {
			return result, fmt.Errorf("failed to get workload status: %w", err)
		}
		_result = append(_result, &v1alpha1.WorkloadState{Namespace: info.Namespace, Name: info.ObjectName(), Ok: ok, Message: strings.TrimSuffix(msg, "\n")})
	}
	sort.SliceStable(_result, func(i, j int) bool {
		return _result[i].String() < _result[j].String()
	})
	result = _result
	return result, nil
}

type WorkloadStateCallbackFunc func(state bool, total int, ready []*v1alpha1.WorkloadState, unready []*v1alpha1.WorkloadState, iteration int) bool

func AreWorkloadsReady(config *Config, callback WorkloadStateCallbackFunc) wait.ConditionWithContextFunc {
	client := config.RESTClient()
	iteration := 0
	return func(ctx context.Context) (bool, error) {
		states, err := client.AllWorkloadStates()
		if err != nil {
			return false, err
		}
		result := true
		var ready, unready []*v1alpha1.WorkloadState
		for _, state := range states {
			if !state.Ok {
				result = false
				unready = append(unready, state)
			} else {
				ready = append(ready, state)
			}
		}
		log.WithFields(log.Fields{
			"total":   len(states),
			"ready":   len(ready),
			"unready": len(unready),
		}).Infof("Workloads total: %d, ready: %d, unready:%d", len(states), len(ready), len(unready))

		if callback != nil {
			result = callback(result, len(states), ready, unready, iteration)
		}
		iteration++

		return result, nil
	}
}

func (config *Config) WaitForWorkloads(ctx context.Context, timeout time.Duration, callback WorkloadStateCallbackFunc) error {
	if timeout > 0 {
		if err := wait.PollUntilContextTimeout(ctx, time.Second*time.Duration(2), timeout, true, AreWorkloadsReady(config, callback)); err != nil {
			return fmt.Errorf("failed to wait for workloads (timeout): %w", err)
		}
		return nil
	} else {
		if err := wait.PollUntilContextCancel(ctx, time.Second*time.Duration(2), true, AreWorkloadsReady(config, callback)); err != nil {
			return fmt.Errorf("failed to wait for workloads: %w", err)
		}
		return nil
	}
}
