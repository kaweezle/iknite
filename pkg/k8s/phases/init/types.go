package init

import (
	"context"

	"github.com/pion/mdns"
	initPhases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/init"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
)

type IkniteClusterProvider interface {
	IkniteCluster() *v1alpha1.IkniteCluster
}

type MDnsConnectionProvider interface {
	MDnsConn() *mdns.Conn
}

type MDnsConnectionHolder interface {
	MDnsConnectionProvider
	SetMDnsConn(conn *mdns.Conn)
}

type KubeletProcessProvider interface {
	KubeletProcess() host.Process
}

type KubeletProcessHolder interface {
	KubeletProcessProvider
	SetKubeletProcess(process host.Process)
}

type StatusServerProvider interface {
	StatusServer() *server.IkniteServer
}

type StatusServerHolder interface {
	StatusServerProvider
	SetStatusServer(srv *server.IkniteServer)
}

type ContextProvider interface {
	Context() context.Context
}

type KustomizeOptionsProvider interface {
	KustomizeOptions() *utils.KustomizeOptions
}

type IkniteInitData interface {
	initPhases.InitData
	IkniteClusterProvider
	MDnsConnectionHolder
	KubeletProcessHolder
	host.HostProvider
	ContextProvider
	StatusServerHolder
	KustomizeOptionsProvider
}
