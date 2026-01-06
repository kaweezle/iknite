package init

import (
	"fmt"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/k8s"
)

func NewPrepareHostPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "prepare-host",
		Short: "Prepare the computer to host Kubernetes.",
		Run:   runPrepareHost,
	}
}

// runPrepare executes the node initialization process.
func runPrepareHost(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	ikniteConfig := &data.IkniteCluster().Spec

	return k8s.PrepareKubernetesEnvironment(ikniteConfig)
}
