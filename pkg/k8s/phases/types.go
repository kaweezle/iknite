package phases

import (
	"context"
	"os/exec"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/pion/mdns"
	phases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/init"
)

type IkniteInitData interface {
	phases.InitData

	IkniteCluster() *v1alpha1.IkniteCluster
	SetKubeletCmd(cmd *exec.Cmd)
	KubeletCmd() *exec.Cmd
	SetMDnsConn(conn *mdns.Conn)
	MDnsConn() *mdns.Conn
	ContextWithCancel() (context.Context, context.CancelFunc)
}
