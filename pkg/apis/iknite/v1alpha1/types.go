package v1alpha1

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	ikniteapi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/constants"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IkniteCluster struct {
	metav1.TypeMeta `json:",inline"`

	Spec IkniteClusterSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// +optional
	Status IkniteClusterStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type IkniteClusterSpec struct {
	// +optional
	Ip net.IP `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip" mapstructure:"ip"`
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty" protobuf:"bytes,2,opt,name=kubernetesVersion" mapstructure:"kubernetes_version"`
	// +optional
	DomainName string `json:"domainName,omitempty" protobuf:"bytes,3,opt,name=domainName" mapstructure:"domain_name"`
	// +optional
	CreateIp bool `json:"createIp,omitempty" protobuf:"bytes,4,opt,name=createIp" mapstructure:"create_ip"`
	// +optional
	NetworkInterface string `json:"networkInterface,omitempty" protobuf:"bytes,5,opt,name=networkInterface" mapstructure:"network_interface"`
	// +optional
	EnableMDNS bool `json:"enableMDNS,omitempty" protobuf:"bytes,6,opt,name=enableMDNS" mapstructure:"enable_mdns"`
	// +optional
	ClusterName string `json:"clusterName,omitempty" protobuf:"bytes,7,opt,name=clusterName" mapstructure:"cluster_name"`
}

func (c *IkniteClusterSpec) GetApiEndPoint() string {
	if c.DomainName != "" {
		return c.DomainName
	}
	return c.Ip.String()
}

type IkniteClusterStatus struct {
	LastUpdateTimeStamp metav1.Time            `json:"lastUpdateTimeStamp" protobuf:"bytes,1,opt,name=lastUpdateTimeStamp"`
	State               ikniteapi.ClusterState `json:"state" protobuf:"bytes,1,opt,name=state"`
	CurrentPhase        string                 `json:"currentPhase" protobuf:"bytes,2,opt,name=currentPhase"`
	WorkloadsState      ClusterWorkloadsState  `json:"workloadsState" protobuf:"bytes,3,opt,name=workloadsState"`
}

type ClusterWorkloadsState struct {
	Count        int              `json:"count" protobuf:"bytes,1,opt,name=count"`
	ReadyCount   int              `json:"readyCount" protobuf:"bytes,2,opt,name=readyCount"`
	UnreadyCount int              `json:"unreadyCount" protobuf:"bytes,3,opt,name=unreadyCount"`
	Ready        []*WorkloadState `json:"ready" protobuf:"bytes,4,opt,name=ready"`
	Unready      []*WorkloadState `json:"unready" protobuf:"bytes,5,opt,name=unready"`
}

type WorkloadState struct {
	Namespace string
	Name      string
	Ok        bool
	Message   string
}

func (r *WorkloadState) LongString() string {

	return fmt.Sprintf("%s %-20s %-54s %s", OkString(r.Ok), r.Namespace, r.Name, r.Message)
}

func (r *WorkloadState) String() string {

	return fmt.Sprintf("%s/%s:%s", r.Namespace, r.Name, OkString(r.Ok))
}

func OkString(b bool) string {
	if b {
		return "🟩"
	}
	return "🟥"
}

func (ikniteCluster *IkniteCluster) Update(state ikniteapi.ClusterState, phase string, ready, unready []*WorkloadState) {
	ikniteCluster.Status.State = state
	ikniteCluster.Status.CurrentPhase = phase
	ikniteCluster.Status.LastUpdateTimeStamp = metav1.Now()
	ikniteCluster.Status.WorkloadsState.Count = len(ready) + len(unready)
	ikniteCluster.Status.WorkloadsState.Ready = ready
	ikniteCluster.Status.WorkloadsState.ReadyCount = len(ready)
	ikniteCluster.Status.WorkloadsState.Unready = unready
	ikniteCluster.Status.WorkloadsState.UnreadyCount = len(unready)
	ikniteCluster.Persist()
}

func (ikniteCluster IkniteCluster) Persist() {
	ikniteClusterJSON, err := json.MarshalIndent(ikniteCluster, "", "  ")
	if err == nil {
		// Write JSON to file
		os.MkdirAll(constants.StatusDirectory, 0755)
		err = os.WriteFile(constants.StatusFile, ikniteClusterJSON, 0644)
		if err != nil {
			log.WithError(err).Warn("Failed to write status.json")
		}
	} else {
		log.WithError(err).Warn("Failed to marshal status.json")
	}
}

func LoadIkniteCluster() (*IkniteCluster, error) {
	ikniteCluster := &IkniteCluster{}
	ikniteClusterJSON, err := os.ReadFile(constants.StatusFile)
	if err == nil {
		err = json.Unmarshal(ikniteClusterJSON, ikniteCluster)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return ikniteCluster, nil
}
