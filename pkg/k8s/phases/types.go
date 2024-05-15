package phases

import (
	"os/exec"

	"github.com/kaweezle/iknite/pkg/k8s"
	phases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/init"
)

type IkniteInitData interface {
	phases.InitData

	IkniteConfig() *k8s.IkniteConfig
	SetKubeletCmd(cmd *exec.Cmd)
	KubeletCmd() *exec.Cmd
}
