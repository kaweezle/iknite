package init

import (
	"fmt"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewPreCleanHostPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "pre-clean-host",
		Short: "Clean leftovers of a previous execution.",
		Run:   runPreCleanHost,
	}
}

type preCleanHostData interface {
	IkniteClusterProvider
	host.HostProvider
	utils.LoggerProvider
}

// runPreCleanHost executes the node initialization process.
func runPreCleanHost(c workflow.RunData) error {
	data, ok := c.(preCleanHostData)
	if !ok {
		return fmt.Errorf("pre-clean host phase invoked with an invalid data struct. ")
	}
	ikniteConfig := &data.IkniteCluster().Spec

	cleaner := k8s.NewCleaner(data.Host(), data.Logger(), ikniteConfig, false)
	if err := cleaner.CleanAll(false, false, true); err != nil {
		return fmt.Errorf("failed to pre-clean host: %w", err)
	}
	return nil
}
