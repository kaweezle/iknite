package init

import (
	"fmt"

	"github.com/kaweezle/iknite/pkg/k8s"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

func NewPreCleanHostPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "pre-clean-host",
		Short: "Clean leftovers of a previous execution.",
		Run:   runPreCleanHost,
	}
}

// runPrepare executes the node initialization process.
func runPreCleanHost(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("pre-clean host phase invoked with an invalid data struct. ")
	}
	ikniteConfig := &data.IkniteCluster().Spec

	return k8s.CleanAll(ikniteConfig, false, true, false)
}
