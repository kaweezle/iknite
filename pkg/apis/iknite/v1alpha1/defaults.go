package v1alpha1

// cSpell: words metav1 apimachinery
import (
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

var KubernetesVersionDefault = constants.KubernetesVersion

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_IkniteClusterSpec(obj *IkniteClusterSpec) {
	// TODO: The defaults should be static and there should be another method
	// that sets dynamic defaults like IPs. This is because the defaulting
	// method is called multiple times and we don't want to override user-set
	// values with defaults on subsequent calls.
	fs := host.NewOsFS()
	wsl := host.IsOnWSL(fs)
	incus := host.IsOnIncus(fs)
	if obj.Ip == nil {
		if wsl || incus {
			obj.Ip = net.ParseIP(constants.WslIPAddress)
		} else {
			obj.Ip, _ = host.NewDefaultNetworkHost().GetOutboundIP() //nolint:errcheck // it fails, no default
		}
	}
	if obj.DomainName == "" && wsl {
		obj.DomainName = constants.WSLHostName
	}
	obj.EnableMDNS = wsl
	if obj.KubernetesVersion == "" {
		obj.KubernetesVersion = KubernetesVersionDefault
	}
	if obj.NetworkInterface == "" {
		obj.NetworkInterface = constants.NetworkInterface
	}
	obj.CreateIp = wsl || incus
	if obj.ClusterName == "" {
		obj.ClusterName = constants.DefaultClusterName
	}
	if obj.Kustomization == "" {
		obj.Kustomization = constants.DefaultKustomization
	}
	if obj.APIBackendDatabaseDirectory == "" {
		obj.APIBackendDatabaseDirectory = constants.KineDirectory
	}
	if obj.StatusServerPort == 0 {
		obj.StatusServerPort = constants.IkniteServerPort
	}
	if obj.StatusUpdateIntervalSeconds == 0 {
		obj.StatusUpdateIntervalSeconds = constants.StatusUpdateIntervalSeconds
	}
	if obj.StatusUpdateLongIntervalSeconds == 0 {
		obj.StatusUpdateLongIntervalSeconds = constants.StatusUpdateLongIntervalSeconds
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
		obj.LastUpdateTimeStamp = metav1.Now().Rfc3339Copy()
	}
}

func SetDefaults_IkniteCluster(obj *IkniteCluster) {
	SetDefaults_IkniteClusterSpec(&obj.Spec)
	SetDefaults_IkniteClusterStatus(&obj.Status)
}
