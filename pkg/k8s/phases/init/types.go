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

type IkniteInitData interface {
	initPhases.InitData

	IkniteCluster() *v1alpha1.IkniteCluster
	SetKubeletProcess(process host.Process)
	KubeletProcess() host.Process
	SetMDnsConn(conn *mdns.Conn)
	MDnsConn() *mdns.Conn
	ContextWithCancel() (context.Context, context.CancelFunc)
	SetStatusServer(srv *server.IkniteServer)
	StatusServer() *server.IkniteServer
	KustomizeOptions() *utils.KustomizeOptions
	Host() host.Host
}
