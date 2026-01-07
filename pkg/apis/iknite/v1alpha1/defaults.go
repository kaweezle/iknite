package v1alpha1

// cSpell: words metav1
// cSpell: disable
import (
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_IkniteClusterSpec(obj *IkniteClusterSpec) {
	wsl := utils.IsOnWSL()
	if obj.Ip == nil {
		if wsl {
			obj.Ip = net.ParseIP(constants.WslIPAddress)
		} else {
			obj.Ip, _ = utils.GetOutboundIP() //nolint:errcheck // it it fails, no default
		}
	}
	if obj.DomainName == "" && wsl {
		obj.DomainName = constants.WSLHostName
	}
	obj.EnableMDNS = wsl
	if obj.KubernetesVersion == "" {
		obj.KubernetesVersion = constants.KubernetesVersion
	}
	if obj.NetworkInterface == "" {
		obj.NetworkInterface = constants.NetworkInterface
	}
	obj.CreateIp = wsl
	if obj.ClusterName == "" {
		obj.ClusterName = constants.DefaultClusterName
	}
	if obj.Kustomization == "" {
		obj.Kustomization = constants.DefaultKustomization
	}
}

func SetDefaults_IkniteClusterStatus(obj *IkniteClusterStatus) {
	if obj.State == iknite.Undefined {
		obj.State = iknite.Stopped
	}
	if obj.CurrentPhase == "" {
		obj.CurrentPhase = "undefined"
	}
	if obj.LastUpdateTimeStamp.IsZero() {
		obj.LastUpdateTimeStamp = metav1.Now()
	}
}

func SetDefaults_IkniteCluster(obj *IkniteCluster) {
	SetDefaults_IkniteClusterSpec(&obj.Spec)
	SetDefaults_IkniteClusterStatus(&obj.Status)
}
