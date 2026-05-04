package init

// cSpell: disable
import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

func NewWorkloadsPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "workloads",
		Short: "Monitor the cluster workloads.",
		Run:   runMonitorWorkloads,
	}
}

type monitorData interface {
	ContextProvider
	IkniteClusterHolder
	RESTClientGetterProvider
	ErrGroupProvider
}

// runPrepare executes the node initialization process.
func runMonitorWorkloads(c workflow.RunData) error {
	data, ok := c.(monitorData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	kubeClient, err := data.RESTClientGetter()
	if err != nil {
		return fmt.Errorf("cannot load the kubernetes configuration: %w", err)
	}

	ctx := data.Context()
	cluster := data.IkniteCluster()

	ticker := time.NewTicker(time.Duration(cluster.Spec.StatusUpdateIntervalSeconds) * time.Second)
	updateWorkloads := k8s.WorkloadsReadyConditionWithContextFunc(kubeClient,
		func(allReady bool, _ int, ready, unready []*v1alpha1.WorkloadState, _, _ int) bool {
			var status iknite.ClusterState
			cluster := data.IkniteCluster()
			if allReady && cluster.Status.State != iknite.Running {
				log.Info("All workloads are ready. Going to 60 seconds interval.")
				ticker.Reset(time.Duration(cluster.Spec.StatusUpdateLongIntervalSeconds) * time.Second)
			}
			if allReady || cluster.Status.State == iknite.Running {
				status = iknite.Running
			} else { // nocov - TODO: add a test case for this branch
				status = iknite.Stabilizing
			}

			data.UpdateIkniteCluster(status, "daemonize", ready, unready)
			return true
		})

	log.Debug("Starting workloads timer...")
	data.ErrGroup().Go(func() error {
		for {
			select {
			case <-ctx.Done():
				log.Info("Workloads monitoring stopped.")
				ticker.Stop()
				return ctx.Err()
			case <-ticker.C:
				log.Debug("Getting workloads information...")
				_, err := updateWorkloads(ctx)
				if err != nil {
					log.Errorf("While getting workloads information: %v", err)
					ticker.Stop()
					return err
				}
			}
		}
	})

	return nil
}
