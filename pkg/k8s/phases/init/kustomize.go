package init

// cSpell: disable
import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
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

// runKustomize is a phase that configures the cluster with base Kustomization.
func runKustomize(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("configure phase invoked with an invalid data struct. ")
	}
	ikniteConfig := data.IkniteCluster().Spec

	force_config := viper.GetBool(config.ForceConfig)
	log.WithFields(log.Fields{
		"force_config":  force_config,
		"kustomization": ikniteConfig.Kustomization,
	}).Info("Performing kustomize configuration")

	k8sConfig, err := k8s.LoadFromDefault()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	// TODO: This probably should be elsewhere
	err = k8sConfig.RenameConfig(ikniteConfig.ClusterName).
		WriteToFile(constants.KubernetesRootConfig)
	if err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}
	if err := k8sConfig.DoKustomization(ikniteConfig.Ip, ikniteConfig.Kustomization, force_config, 0); err != nil {
		return fmt.Errorf("failed to apply kustomization: %w", err)
	}
	return nil
}
