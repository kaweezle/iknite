package init

// cSpell: disable
import (
	"fmt"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

func NewKustomizeClusterPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "kustomize-cluster",
		Short: "Configure the cluster with base Kustomization.",
		Run:   runKustomize,
	}
}

type kustomizeData interface {
	host.HostProvider
	KustomizeOptionsProvider
	ContextProvider
	RESTClientGetterProvider
}

// runKustomize is a phase that configures the cluster with base Kustomization.
func runKustomize(c workflow.RunData) error {
	data, ok := c.(kustomizeData)
	if !ok {
		return fmt.Errorf("configure phase invoked with an invalid data struct. ")
	}

	kubeClient, err := data.RESTClientGetter()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	ctx := data.Context()
	kustomizeOptions := data.KustomizeOptions()
	if err := k8s.Kustomize(
		ctx,
		kubeClient,
		data.Host(),
		kustomizeOptions,
	); err != nil {
		return fmt.Errorf("failed to apply kustomization: %w", err)
	}
	return nil
}
