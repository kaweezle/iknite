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
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var Scheme = runtime.NewScheme()

//nolint:gochecknoinits // This is a test utility package, so it's fine to have an init function here.
func init() {
	utilruntime.Must(corev1.AddToScheme(Scheme))
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

// hardcoded is good enough for the test we're running.
var rootScopedKinds = map[schema.GroupKind]bool{
	{Group: "admission.k8s.io", Kind: "AdmissionReview"}: true,

	{Group: "admissionregistration.k8s.io", Kind: "ValidatingWebhookConfiguration"}: true,
	{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"}:   true,

	{Group: "authentication.k8s.io", Kind: "TokenReview"}: true,

	{Group: "authorization.k8s.io", Kind: "SubjectAccessReview"}:     true,
	{Group: "authorization.k8s.io", Kind: "SelfSubjectAccessReview"}: true,
	{Group: "authorization.k8s.io", Kind: "SelfSubjectRulesReview"}:  true,

	{Group: "certificates.k8s.io", Kind: "CertificateSigningRequest"}: true,

	{Group: "", Kind: "Node"}:             true,
	{Group: "", Kind: "Namespace"}:        true,
	{Group: "", Kind: "PersistentVolume"}: true,
	{Group: "", Kind: "ComponentStatus"}:  true,

	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:        true,
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"}: true,

	{Group: "scheduling.k8s.io", Kind: "PriorityClass"}: true,

	{Group: "storage.k8s.io", Kind: "StorageClass"}:     true,
	{Group: "storage.k8s.io", Kind: "VolumeAttachment"}: true,

	{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}: true,

	{Group: "apiserver.k8s.io", Kind: "AdmissionConfiguration"}: true,

	{Group: "audit.k8s.io", Kind: "Event"}:  true,
	{Group: "audit.k8s.io", Kind: "Policy"}: true,

	{Group: "apiregistration.k8s.io", Kind: "APIService"}: true,

	{Group: "metrics.k8s.io", Kind: "NodeMetrics"}: true,
}
