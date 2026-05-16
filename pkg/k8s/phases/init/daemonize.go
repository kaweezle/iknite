package init

// cSpell: disable
import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func NewDaemonizePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "daemonize",
		Short: "Wait for the kubelet to stop.",
		Run:   runDaemonize,
	}
}

func WaitForKubelet(ctx context.Context, process host.Process, logger logrus.FieldLogger) error {
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- process.Wait()
	}()

	var err error
	// Wait for the signals or for the child process to stop
	logger.Info("Waiting for the kubelet to stop or receive a stop signal...")
	for alive := true; alive; {
		select {
		case <-ctx.Done():
			// Stop the cmd process
			logger.Info("Received TERM Signal. Stopping kubelet...")
			err = host.TerminateProcess(process, &alive)

		case err = <-cmdDone:
			// Child process has stopped
			logger.Infof("Kubelet stopped with state: %s", process.State().String())
			alive = false
		}
	}

	if err != nil {
		return fmt.Errorf("failed to wait for kubelet: %w", err)
	}

	return nil
}

type daemonizeData interface {
	KubeletProcessHolder
	IkniteClusterHolder
	host.HostProvider
	ContextProvider
	ShutdownHookRunner
	utils.LoggerProvider
}

// runPrepare executes the node initialization process.
func runDaemonize(c workflow.RunData) error {
	data, ok := c.(daemonizeData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	kubeletProcess := data.KubeletProcess()
	if kubeletProcess == nil {
		return nil
	}
	logger := data.Logger()

	err := WaitForKubelet(data.Context(), kubeletProcess, logger)

	data.UpdateIkniteCluster(iknite.Stopping, "stop", nil, nil)
	if err == nil {
		// Prevent double stop
		data.SetKubeletProcess(nil)
	} else {
		logger.WithError(err).Warn("Error while waiting for kubelet to stop")
	}

	err = data.RunShutdownHooks()
	if err != nil {
		logger.WithError(err).Warn("Error running shutdown hooks")
	}

	err = k8s.CleanAll(data.Host(), &data.IkniteCluster().Spec, true, false, false, false)
	if err != nil {
		logger.WithError(err).Warn("Error during cleanup after kubelet stopped")
	}
	data.UpdateIkniteCluster(iknite.Stopped, "", nil, nil)
	return nil
}
