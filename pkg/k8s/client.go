package k8s

// cSpell: words clientcmd clientconfig restconfig casttype metav1 polymorphichelpers restmapper genericclioptions
// cSpell: words testutil serviceaccount corev
// cSpell: disable
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

// cSpell: enable

type Client struct {
	clientconfig clientcmd.ClientConfig
	restConfig   *rest.Config
	mapper       meta.RESTMapper
}

var _ genericclioptions.RESTClientGetter = (*Client)(nil)

func NewClientFromFile(fs host.FileSystem, path string) (*Client, error) {
	content, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}
	clientconfig, err := clientcmd.NewClientConfigFromBytes(content)
	if err != nil {
		return nil, fmt.Errorf("failed to create client config from bytes: %w", err)
	}
	return NewClientFromClientConfig(clientconfig), nil
}

func NewDefaultClient(fs host.FileSystem) (*Client, error) {
	return NewClientFromFile(fs, kubeadmConstants.GetAdminKubeConfigPath())
}

func NewClientFromConfig(config *api.Config) *Client {
	return NewClientFromClientConfig(clientcmd.NewDefaultClientConfig(*config, nil))
}

func NewClientFromClientConfig(config clientcmd.ClientConfig) *Client {
	return &Client{clientconfig: config}
}

func NewClientFromRestConfig(restConfig *rest.Config) *Client {
	return &Client{restConfig: restConfig}
}

func (r *Client) ToRESTConfig() (*rest.Config, error) {
	if r.restConfig != nil {
		result := *r.restConfig
		return &result, nil
	}

	if r.clientconfig == nil {
		return nil, fmt.Errorf("client configuration is not set")
	}
	restConfig, err := r.clientconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}
	r.restConfig = restConfig
	return restConfig, nil
}

func (r *Client) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restconfig, err := r.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config for discovery: %w", err)
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	return memory.NewMemCacheClient(dc), nil
}

func ClientSet(client resource.RESTClientGetter) (kubernetes.Interface, error) {
	restconfig, err := client.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config for Kubernetes client: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return k8sClient, nil
}

func (r *Client) ToRESTMapper() (meta.RESTMapper, error) {
	if r.mapper != nil {
		return r.mapper, nil
	}
	// Allow using test RESTMapper if set as an extension in the client config, which is useful for testing with a
	// mock RESTMapper without needing to set up a fake API server.
	if r.clientconfig != nil {
		mapper, err := testutil.GetTestMapperFromClientConfig(r.clientconfig)
		if err != nil && !errors.Is(err, testutil.ErrNoTestMapper) {
			return nil, fmt.Errorf("failed to get test REST mapper from client config extension: %w", err)
		}
		if mapper != nil {
			r.mapper = mapper
			return mapper, nil
		}
	}
	dc, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get discovery client for REST mapper: %w", err)
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func copyConfig(from *rest.Config) *rest.Config {
	config := *from
	config.APIPath = ""
	config.GroupVersion = nil
	if config.Timeout == 0 {
		config.Timeout = 15 * time.Second
	}
	if config.Burst == 0 {
		config.Burst = 300
	}
	config.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return &config
}

func RESTClientFromConfig(restConfig *rest.Config) (rest.Interface, error) {
	config := copyConfig(restConfig)
	result, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}
	return result, nil
}

func RESTClient(r resource.RESTClientGetter) (rest.Interface, error) {
	restconfig, err := r.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config for discovery: %w", err)
	}
	return RESTClientFromConfig(restconfig)
}

func (r *Client) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	if r.clientconfig != nil {
		return r.clientconfig
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

func ResourceInfosFromResMap(client resource.RESTClientGetter, resources resmap.ResMap) ([]*resource.Info, error) {
	rawResources, err := resources.AsYaml()
	if err != nil { // nocov - unlikely to fail as the resources are already in memory and should be well-formed
		return nil, fmt.Errorf("failed to convert resources to YAML: %w", err)
	}

	result := resource.NewBuilder(client).
		Unstructured().
		ContinueOnError().
		Stream(bytes.NewBufferString(string(rawResources)), "kustomize").
		Flatten().
		Do()

	infos, err := result.Infos()
	if err != nil {
		return nil, fmt.Errorf("failed to build resource infos: %w", err)
	}

	return infos, nil
}

func ApplyResourceInfosServerSide(infos []*resource.Info) error {
	force := true
	for _, info := range infos {
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
		if err != nil { // nocov - unlikely to fail as the objects should be well-formed and convertible
			return fmt.Errorf("failed to convert resource %s to unstructured: %w", info.ObjectName(), err)
		}

		payload, err := json.Marshal(unstructuredObj)
		if err != nil { // nocov - unlikely to fail as the objects should be well-formed and convertible
			return fmt.Errorf("failed to marshal resource %s for apply: %w", info.ObjectName(), err)
		}

		helper := resource.NewHelper(info.Client, info.Mapping).WithFieldManager("iknite")
		if _, err = helper.Patch(
			info.Namespace,
			info.Name,
			k8sTypes.ApplyPatchType,
			payload,
			&metav1.PatchOptions{Force: &force, FieldManager: "iknite"},
		); err != nil {
			return fmt.Errorf("failed to server-side apply resource %s: %w", info.ObjectName(), err)
		}
	}

	return nil
}

var ApplicationSchemaGroupVersionKind = schema.GroupVersionKind{
	Group:   "argoproj.io",
	Version: "v1alpha1",
	Kind:    "Application",
}

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
	Sync   SyncStatus   `json:"sync,omitempty"   protobuf:"bytes,2,opt,name=sync"`
	Health HealthStatus `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`
}

type Application struct {
	Status            ApplicationStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
	metav1.TypeMeta   `                  json:",inline"`
	metav1.ObjectMeta `                  json:"metadata"         protobuf:"bytes,1,opt,name=metadata"`
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

func (s *ApplicationStatusViewer) Status(
	obj runtime.Unstructured,
	_ int64,
) (string, bool, error) {
	application := &Application{}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		obj.UnstructuredContent(),
		application,
	)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert %T to %T: %w", obj, application, err)
	}

	healthStatusString := application.Status.Health.Status
	syncStatusString := application.Status.Sync.Status

	message := fmt.Sprintf("application %q sync status: %s, health status: %s",
		application.Name, syncStatusString, healthStatusString)
	return message, healthStatusString == "Healthy" && syncStatusString == "Synced", nil
}

// ApplyResMapWithServerSideApply applies the given resources to the cluster using server-side apply. It returns the
// IDs of the applied resources or an error if the operation fails.
func ApplyResMapWithServerSideApply(client resource.RESTClientGetter, resources resmap.ResMap) ([]resid.ResId, error) {
	ids := resources.AllIds()

	// Separate cluster-scoped resources from namespace-scoped ones, as the former need to be applied before the latter
	// to avoid potential dependency issues.
	clusterResources := resmap.NewFactory(provider.NewDefaultDepProvider().GetResourceFactory()).
		FromResourceSlice(resources.ClusterScoped())
	if clusterResources.Size() != 0 {
		clusterInfos, err := ResourceInfosFromResMap(client, clusterResources)
		if err != nil {
			return nil, fmt.Errorf("failed to build cluster resource infos: %w", err)
		}
		if err = ApplyResourceInfosServerSide(clusterInfos); err != nil {
			return nil, fmt.Errorf("failed to apply cluster resources: %w", err)
		}

		// Remove cluster-scoped resources from the original resmap to avoid applying them again in the next step.
		for _, curID := range clusterResources.AllIds() {
			if err = resources.Remove(curID); err != nil {
				return nil, fmt.Errorf("failed to remove cluster-scoped resource: %w", err)
			}
		}
	}

	// Apply namespace-scoped resources after cluster-scoped ones.
	if resources.Size() != 0 {
		resourceInfos, err := ResourceInfosFromResMap(client, resources)
		if err != nil {
			return nil, fmt.Errorf("failed to build resource infos: %w", err)
		}
		if err = ApplyResourceInfosServerSide(resourceInfos); err != nil {
			return nil, fmt.Errorf("failed to apply resources: %w", err)
		}
	}

	return ids, nil
}

func AllWorkloadStates(client resource.RESTClientGetter) ([]*v1alpha1.WorkloadState, error) {
	resourceTypes := []string{"deployments", "statefulsets", "daemonsets", "applications"}

	mapper, err := client.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST mapper: %w", err)
	}
	validResourceTypes, err := ValidateResourceTypes(mapper, resourceTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to validate resource types: %w", err)
	}

	r := resource.NewBuilder(client).
		Unstructured().
		AllNamespaces(true).
		ResourceTypes(validResourceTypes...).
		SelectAllParam(true).
		ContinueOnError().
		Flatten().
		Do()

	var infos []*resource.Info
	if infos, err = r.Infos(); err != nil {
		return nil, fmt.Errorf("failed to get resource infos: %w", err)
	}

	result := make([]*v1alpha1.WorkloadState, 0, len(infos))

	for _, info := range infos {
		// We asked for unstructured objects, so this should never fail.
		u, ok := any(info.Object).(*unstructured.Unstructured)
		if !ok { // nocov - this should never happen as the builder was set to return unstructured objects
			return nil, fmt.Errorf("failed to cast object to unstructured: %T", info.Object)
		}

		var v polymorphichelpers.StatusViewer
		if v, err = StatusViewerFor(u.GroupVersionKind().GroupKind()); err != nil {
			return nil, fmt.Errorf("failed to get status viewer: %w", err)
		}

		var msg string
		if msg, ok, err = v.Status(u, 0); err != nil {
			return nil, fmt.Errorf("failed to get workload status: %w", err)
		}
		result = append(result, &v1alpha1.WorkloadState{
			Namespace: info.Namespace,
			Name:      info.ObjectName(),
			Ok:        ok,
			Message:   strings.TrimSuffix(msg, "\n"),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].String() < result[j].String()
	})
	return result, nil
}

type WorkloadStateCallbackFunc func(allReady bool, total int, ready []*v1alpha1.WorkloadState,
	unready []*v1alpha1.WorkloadState, iteration, okIterations int) bool

func WorkloadsReadyConditionWithContextFunc(
	client resource.RESTClientGetter,
	callback WorkloadStateCallbackFunc,
) wait.ConditionWithContextFunc {
	iteration := 0
	okIterations := 0
	return func(_ context.Context) (bool, error) {
		states, err := AllWorkloadStates(client)
		if err != nil {
			return false, err
		}
		allReady := true
		var ready, unready []*v1alpha1.WorkloadState
		for _, state := range states {
			if !state.Ok {
				allReady = false
				okIterations = 0
				unready = append(unready, state)
			} else {
				ready = append(ready, state)
			}
		}
		log.WithFields(log.Fields{
			"total":        len(states),
			"ready":        len(ready),
			"unready":      len(unready),
			"okIterations": okIterations,
		}).Infof("Workloads total: %d, ready: %d, unready:%d, okIterations: %d", len(states), len(ready), len(unready),
			okIterations)
		if allReady {
			okIterations++
		}

		if callback != nil {
			allReady = callback(allReady, len(states), ready, unready, iteration, okIterations)
		}
		iteration++

		return allReady, nil
	}
}

func InClusterConfig(fs host.FileSystem) (*rest.Config, error) {
	const (
		tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // From client-go
		rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	)
	svcHost, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if svcHost == "" || port == "" {
		return nil, rest.ErrNotInCluster
	}

	token, err := fs.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("while reading token file %s: %w", tokenFile, err)
	}

	tlsClientConfig := rest.TLSClientConfig{}

	pemBlock, err := fs.ReadFile(rootCAFile)
	if err != nil {
		log.Errorf("Expected to load root CA config from %s, but got err: %v", rootCAFile, err)
	} else {
		_, err := cert.NewPoolFromBytes(pemBlock)
		if err != nil {
			log.Errorf("Expected to parse root CA config from %s, but got err: %v", rootCAFile, err)
		} else {
			// Only set the CA data if it can be parsed successfully,
			// otherwise the client will fail to connect to the API server.
			tlsClientConfig.CAData = pemBlock
		}
	}

	return &rest.Config{
		// TODO: switch to using cluster DNS.
		Host:            "https://" + net.JoinHostPort(svcHost, port),
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenFile,
	}, nil
}
