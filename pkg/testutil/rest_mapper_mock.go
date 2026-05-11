// cSpell: words testutil corev1 apiextensionsv1 apiextensions apimachinery utilruntime admissionregistration
// cSpell: words apiregistration
package testutil

import (
	argoprojv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var Scheme = runtime.NewScheme()

//nolint:gochecknoinits // This is a test utility package, so it's fine to have an init function here.
func init() {
	utilruntime.Must(corev1.AddToScheme(Scheme))
	utilruntime.Must(corev1.AddToScheme(latest.Scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(Scheme))
	utilruntime.Must(argoprojv1.AddToScheme(Scheme))
	utilruntime.Must(appsv1.AddToScheme(Scheme))
	utilruntime.Must(batchv1.AddToScheme(Scheme))
	utilruntime.Must(rbacv1.AddToScheme(Scheme))
	utilruntime.Must(storagev1.AddToScheme(Scheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(Scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(Scheme))
}

func NewRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper(Scheme.PreferredVersionAllGroups())
	// enumerate all supported versions, get the kinds, and register with the mapper how to address
	// our resources.
	for _, gv := range Scheme.PreferredVersionAllGroups() {
		for kind := range Scheme.KnownTypes(gv) {
			scope := meta.RESTScopeNamespace
			if rootScopedKinds[gv.WithKind(kind).GroupKind()] {
				scope = meta.RESTScopeRoot
			}
			mapper.Add(gv.WithKind(kind), scope)
		}
	}
	return mapper
}

const authorizationGroupName = "authorization.k8s.io"

// hardcoded is good enough for the test we're running.
// TODO: There are items here that are not part of the test mapper. Clean it or add the missing items to the test.
var rootScopedKinds = map[schema.GroupKind]bool{
	{Group: "admission.k8s.io", Kind: "AdmissionReview"}: true,

	{Group: admissionregistrationv1.GroupName, Kind: "ValidatingWebhookConfiguration"}: true,
	{Group: admissionregistrationv1.GroupName, Kind: "MutatingWebhookConfiguration"}:   true,

	{Group: "authentication.k8s.io", Kind: "TokenReview"}: true,

	{Group: authorizationGroupName, Kind: "SubjectAccessReview"}:     true,
	{Group: authorizationGroupName, Kind: "SelfSubjectAccessReview"}: true,
	{Group: authorizationGroupName, Kind: "SelfSubjectRulesReview"}:  true,

	{Group: "certificates.k8s.io", Kind: "CertificateSigningRequest"}: true,

	{Group: "", Kind: "Node"}:             true,
	{Group: "", Kind: "Namespace"}:        true,
	{Group: "", Kind: "PersistentVolume"}: true,
	{Group: "", Kind: "ComponentStatus"}:  true,

	{Group: rbacv1.GroupName, Kind: "ClusterRole"}:        true,
	{Group: rbacv1.GroupName, Kind: "ClusterRoleBinding"}: true,

	{Group: "scheduling.k8s.io", Kind: "PriorityClass"}: true,

	{Group: storagev1.GroupName, Kind: "StorageClass"}:     true,
	{Group: storagev1.GroupName, Kind: "VolumeAttachment"}: true,

	{Group: apiextensionsv1.GroupName, Kind: "CustomResourceDefinition"}: true,

	{Group: "apiserver.k8s.io", Kind: "AdmissionConfiguration"}: true,

	{Group: "audit.k8s.io", Kind: "Event"}:  true,
	{Group: "audit.k8s.io", Kind: "Policy"}: true,

	{Group: apiregistrationv1.GroupName, Kind: "APIService"}: true,

	{Group: "metrics.k8s.io", Kind: "NodeMetrics"}: true,
}
