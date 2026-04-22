package init

import (
	"fmt"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

func NewPrepareHostPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "prepare-host",
		Short: "Prepare the computer to host Kubernetes.",
		Run:   runPrepareHost,
	}
}

type prepareHostData interface {
	IkniteClusterProvider
	host.HostProvider
	ContextProvider
}

// runPrepare executes the node initialization process.
func runPrepareHost(c workflow.RunData) error {
	data, ok := c.(prepareHostData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	ikniteConfig := &data.IkniteCluster().Spec
	alpineHost := data.Host()

	if err := k8s.PrepareKubernetesEnvironment(data.Context(), alpineHost, ikniteConfig); err != nil {
		return fmt.Errorf("failed to prepare kubernetes environment: %w", err)
	}
	return nil
}
