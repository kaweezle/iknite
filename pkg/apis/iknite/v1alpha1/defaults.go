package v1alpha1

import (
	"net"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_IkniteClusterSpec(obj *IkniteClusterSpec) {
	wsl := utils.IsOnWSL()
	if obj.Ip == nil {
		if wsl {
			obj.Ip = net.ParseIP(constants.WSLIPAddress)
		} else {
			obj.Ip, _ = utils.GetOutboundIP()
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
}

func SetDefautls_IkniteClusterStatus(obj *IkniteClusterStatus) {
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
	SetDefautls_IkniteClusterStatus(&obj.Status)
}
