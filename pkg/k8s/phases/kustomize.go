package phases

// cSpell: disable
import (
	"fmt"

	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	log "github.com/sirupsen/logrus"
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

	config, err := k8s.LoadFromDefault()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}
	// TODO: This probably should be elsewhere
	err = config.RenameConfig(ikniteConfig.ClusterName).WriteToFile(constants.KubernetesRootConfig)
	if err != nil {
		return errors.Wrap(err, "failed to write configuration")
	}
	return config.DoKustomization(ikniteConfig.Ip, ikniteConfig.Kustomization, force_config, 0)
}
