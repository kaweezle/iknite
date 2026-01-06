package reset

// cSpell: disable
import (
	"io"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"

	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
)

// cSpell: enable

type IkniteResetData interface {
	// From k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/reset.resetData
	ForceReset() bool
	InputReader() io.Reader
	IgnorePreflightErrors() sets.Set[string]
	Cfg() *kubeadmApi.InitConfiguration
	ResetCfg() *kubeadmApi.ResetConfiguration
	DryRun() bool
	Client() clientset.Interface
	CertificatesDir() string
	CRISocketPath() string
	CleanupTmpDir() bool

	IkniteCluster() *v1alpha1.IkniteCluster
}
