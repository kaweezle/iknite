package init

// cSpell: disable
import (
	"context"
	"fmt"

	"github.com/pion/mdns"
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

func WaitForKubelet(ctx context.Context, process host.Process, conn *mdns.Conn) error {
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

	if err == nil && conn != nil {
		log.Info("Stopping the mdns responder...")
		err = conn.Close()
		if err != nil {
			return fmt.Errorf("failed to close mdns responder: %w", err)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to wait for kubelet: %w", err)
	}

	return nil
}

// runPrepare executes the node initialization process.
func runDaemonize(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	kubeletProcess := data.KubeletProcess()
	if kubeletProcess == nil {
		return nil
	}
	conn := data.MDnsConn()
	ctx, _ := data.ContextWithCancel()

	err := WaitForKubelet(ctx, kubeletProcess, conn)

	data.IkniteCluster().Update(iknite.Stopping, "stop", nil, nil, data.Host())
	if err == nil {
		// Prevent double stop
		data.SetKubeletProcess(nil)
	}

	ensureServerStopped(data)

	data.IkniteCluster().Update(iknite.Cleaning, "clean", nil, nil, data.Host())
	err = k8s.CleanAll(data.Host(), &data.IkniteCluster().Spec, true, false, false, false)
	if err != nil {
		log.WithError(err).Warn("Error during cleanup after kubelet stopped")
	}
	data.IkniteCluster().Update(iknite.Stopped, "", nil, nil, data.Host())
	return nil
}
