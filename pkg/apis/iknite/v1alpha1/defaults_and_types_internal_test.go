// cSpell: words paralleltest apimachinery metav1 ikniteapi
package v1alpha1

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ikniteapi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

func TestSetDefaults_IkniteClusterSpec(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	spec := &IkniteClusterSpec{Ip: []byte{127, 0, 0, 1}}
	SetDefaults_IkniteClusterSpec(spec)

	req.NotEmpty(spec.KubernetesVersion)
	req.NotEmpty(spec.NetworkInterface)
	req.NotEmpty(spec.ClusterName)
	req.NotEmpty(spec.Kustomization)
	req.NotEmpty(spec.APIBackendDatabaseDirectory)
	req.NotZero(spec.StatusServerPort)
	fs := host.NewOsFS()
	req.Equal(host.IsOnWSL(fs), spec.EnableMDNS)
	req.Equal(host.IsOnWSL(fs) || host.IsOnIncus(fs), spec.CreateIp)
}

func TestSetDefaults_IkniteClusterStatusAndCluster(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	status := &IkniteClusterStatus{}
	SetDefaults_IkniteClusterStatus(status)
	req.Equal(ikniteapi.Stopped, status.State)
	req.Equal("undefined", status.CurrentPhase)
	req.False(status.LastUpdateTimeStamp.IsZero())

	cluster := &IkniteCluster{}
	SetDefaults_IkniteCluster(cluster)
	req.NotEmpty(cluster.Spec.KubernetesVersion)
	req.Equal(ikniteapi.Stopped, cluster.Status.State)
}

func TestIkniteClusterSpec_GetApiEndPoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		spec IkniteClusterSpec
	}{
		{
			name: "domain has priority",
			spec: IkniteClusterSpec{DomainName: "iknite.local", Ip: []byte{10, 0, 0, 2}},
			want: "iknite.local",
		},
		{name: "fallback to ip", spec: IkniteClusterSpec{Ip: []byte{10, 0, 0, 3}}, want: "10.0.0.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			req.Equal(tt.want, tt.spec.GetApiEndPoint())
		})
	}
}

func TestWorkloadStateStringHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	workload := &WorkloadState{Namespace: "kube-system", Name: "coredns", Message: "Ready", Ok: true}
	req.Contains(workload.String(), "kube-system/coredns")
	req.Contains(workload.LongString(), "kube-system")
	req.Contains(workload.LongString(), "Ready")
	req.NotEqual(OkString(true), OkString(false))
}

func TestIkniteCluster_Update(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cluster := &IkniteCluster{}
	SetDefaults_IkniteCluster(cluster)
	fs := host.NewMemMapFS()

	ready := []*WorkloadState{{Namespace: "ns", Name: "a", Ok: true, Message: "ok"}}
	unready := []*WorkloadState{{Namespace: "ns", Name: "b", Ok: false, Message: "waiting"}}
	cluster.Update(ikniteapi.Stabilizing, "phase-a", ready, unready)
	cluster.Persist(fs)

	req.Equal(ikniteapi.Stabilizing, cluster.Status.State)
	req.Equal("phase-a", cluster.Status.CurrentPhase)
	req.Equal(2, cluster.Status.WorkloadsState.Count)
	req.Equal(1, cluster.Status.WorkloadsState.ReadyCount)
	req.Equal(1, cluster.Status.WorkloadsState.UnreadyCount)
	req.Equal(ready, cluster.Status.WorkloadsState.Ready)
	req.Equal(unready, cluster.Status.WorkloadsState.Unready)
	exist, err := fs.Exists(constants.StatusFile)
	req.NoError(err)
	req.True(exist)
}

func TestLoadIkniteClusterErrors(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	_, err := LoadIkniteCluster(fs)
	req.Error(err)
	req.True(os.IsNotExist(err))

	cluster, err := LoadIkniteClusterOrDefault(fs)
	req.NoError(err)
	req.NotNil(cluster)
	req.Equal(ikniteapi.Stopped, cluster.Status.State)

	_, err = os.Stat("/this/path/should/not/exist")
	req.ErrorIs(err, os.ErrNotExist)
}

func TestRegisterAndSchemeHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	req.Equal(ikniteapi.GroupName, SchemeGroupVersion.Group)
	req.Equal(ikniteapi.V1alpha1Version, SchemeGroupVersion.Version)
	req.Equal("Thing", Kind("Thing").Kind)
	req.Equal("items", Resource("items").Resource)

	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	req.NoError(err)

	obj, err := scheme.New(SchemeGroupVersion.WithKind(ikniteapi.IkniteClusterKind))
	req.NoError(err)
	req.IsType(&IkniteCluster{}, obj)

	status := &IkniteClusterStatus{LastUpdateTimeStamp: metav1.Now()}
	statusCopy := status.DeepCopy()
	req.NotSame(status, statusCopy)
	req.Equal(status.LastUpdateTimeStamp, statusCopy.LastUpdateTimeStamp)
}
