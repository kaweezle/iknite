// cSpell: words genericclioptions
package init

import (
	"context"

	"github.com/pion/mdns"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	initPhases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/init"

	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
)

type IkniteClusterProvider interface {
	IkniteCluster() *v1alpha1.IkniteCluster
}

type IkniteClusterUpdater interface {
	UpdateIkniteCluster(state ikniteApi.ClusterState, phase string, ready, unready []*v1alpha1.WorkloadState)
}

type IkniteClusterHolder interface {
	IkniteClusterProvider
	IkniteClusterUpdater
}

type MDnsConnectionCloser interface {
	CloseMDnsConn() error
}

type MDnsConnectionHolder interface {
	MDnsConnectionCloser
	SetMDnsConn(conn *mdns.Conn)
}

type KubeletProcessProvider interface {
	KubeletProcess() host.Process
}

type KubeletProcessHolder interface {
	KubeletProcessProvider
	SetKubeletProcess(process host.Process)
}

type StatusServerSetter interface {
	SetStatusServer(srv *server.IkniteServer)
}

type StatusServerStopper interface {
	StopStatusServer() error
}

type ContextProvider interface {
	Context() context.Context
}

type KustomizeOptionsProvider interface {
	KustomizeOptions() *utils.KustomizeOptions
}

type RESTClientGetterProvider interface {
	RESTClientGetter() (genericclioptions.RESTClientGetter, error)
}

type ManifestDirProvider interface {
	ManifestDir() string
}

type IkniteInitData interface {
	initPhases.InitData
	IkniteClusterHolder
	MDnsConnectionHolder
	KubeletProcessHolder
	host.HostProvider
	ContextProvider
	StatusServerSetter
	StatusServerStopper
	KustomizeOptionsProvider
	RESTClientGetterProvider
}
