package phases

import (
	"fmt"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	log "github.com/sirupsen/logrus"
)

func NewConfigureClusterPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "kustomize-cluster",
		Short: "Configure the cluster with base Kustomization.",
		Run:   runConfigure,
	}
}

// runPrepare executes the node initialization process.
func runConfigure(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("configure phase invoked with an invalid data struct. ")
	}
	ikniteConfig := data.IkniteConfig()

	force_config := viper.GetBool("force_config")
	log.Info("Performing kustomize configuration")

	config, err := k8s.LoadFromDefault()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}
	// TODO: This probably should be elsewhere
	err = config.RenameConfig(ikniteConfig.ClusterName).WriteToFile(constants.KubernetesRootConfig)
	if err != nil {
		return errors.Wrap(err, "failed to write configuration")
	}
	return config.DoConfiguration(ikniteConfig.Ip, force_config, 0)
}
