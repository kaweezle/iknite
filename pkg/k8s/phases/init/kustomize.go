package init

// cSpell: disable
import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/config"
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
	IkniteClusterProvider
	host.HostProvider
	KustomizeOptionsProvider
	ContextProvider
}

// runKustomize is a phase that configures the cluster with base Kustomization.
func runKustomize(c workflow.RunData) error {
	data, ok := c.(kustomizeData)
	if !ok {
		return fmt.Errorf("configure phase invoked with an invalid data struct. ")
	}
	ikniteConfig := data.IkniteCluster().Spec

	force_config := viper.GetBool(config.ForceConfig)
	log.WithFields(log.Fields{
		"force_config":  force_config,
		"kustomization": ikniteConfig.Kustomization,
	}).Info("Performing kustomize configuration")

	kubeClient, err := k8s.NewDefaultClient(data.Host())
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	ctx := data.Context()
	kustomizeOptions := data.KustomizeOptions()
	if err := k8s.Kustomize(
		ctx,
		kubeClient,
		data.Host(),
		ikniteConfig.Kustomization,
		kustomizeOptions,
	); err != nil {
		return fmt.Errorf("failed to apply kustomization: %w", err)
	}
	return nil
}
