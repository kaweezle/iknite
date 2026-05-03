package init

// cSpell: disable
import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

func NewDaemonizePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "daemonize",
		Short: "Wait for the kubelet to stop.",
		Run:   runDaemonize,
	}
}

func WaitForKubelet(ctx context.Context, process host.Process) error {
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- process.Wait()
	}()

	var err error
	// Wait for the signals or for the child process to stop
	log.Info("Waiting for the kubelet to stop or receive a stop signal...")
	for alive := true; alive; {
		select {
		case <-ctx.Done():
			// Stop the cmd process
			log.Info("Received TERM Signal. Stopping kubelet...")
			err = host.TerminateProcess(process, &alive)

		case err = <-cmdDone:
			// Child process has stopped
			log.Infof("Kubelet stopped with state: %s", process.State().String())
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
	MDnsConnectionCloser
	IkniteClusterHolder
	host.HostProvider
	ContextProvider
	StatusServerStopper
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

	err := WaitForKubelet(data.Context(), kubeletProcess)

	data.UpdateIkniteCluster(iknite.Stopping, "stop", nil, nil)
	if err == nil {
		// Prevent double stop
		data.SetKubeletProcess(nil)
	} else {
		log.WithError(err).Warn("Error while waiting for kubelet to stop")
	}

	err = data.CloseMDnsConn()
	if err != nil {
		log.WithError(err).Warn("Error closing mdns connection")
	}

	err = data.StopStatusServer()
	if err != nil {
		log.WithError(err).Warn("Error stopping iknite status server")
	}

	err = k8s.CleanAll(data.Host(), &data.IkniteCluster().Spec, true, false, false, false)
	if err != nil {
		log.WithError(err).Warn("Error during cleanup after kubelet stopped")
	}
	data.UpdateIkniteCluster(iknite.Stopped, "", nil, nil)
	return nil
}
