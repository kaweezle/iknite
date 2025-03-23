package init

// cSpell: disable
import (
	"fmt"
	"time"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

// cSpell: enable

func NewWorkloadsPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "workloads",
		Short: "Monitor the cluster workloads.",
		Run:   runMonitorWorkloads,
	}
}

// runPrepare executes the node initialization process.
func runMonitorWorkloads(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	cluster := data.IkniteCluster()
	ctx, _ := data.ContextWithCancel()

	ticker := time.NewTicker(5 * time.Second)
	config, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	if err != nil {
		return errors.Wrap(err, "Cannot load the kubernetes configuration")
	}
	updateWorkloads := k8s.AreWorkloadsReady(config, func(state bool, total int, ready, unready []*v1alpha1.WorkloadState) bool {
		var status iknite.ClusterState
		if state && cluster.Status.State != iknite.Running {
			log.Info("All workloads are ready. Going to 60 seconds interval.")
			ticker.Reset(60 * time.Second)
		}
		if state || cluster.Status.State == iknite.Running {
			status = iknite.Running
		} else {
			status = iknite.Stabilizing
		}

		cluster.Update(status, "daemonize", ready, unready)
		return true
	})

	log.Debug("Starting workloads timer...")
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("Workloads monitoring stopped.")
				ticker.Stop()
				return
			case <-ticker.C:
				log.Debug("Getting workloads information...")
				updateWorkloads(ctx)
			}
		}

	}()

	return nil
}
