package init

// cSpell: disable
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

func WaitForKubelet(process host.Process, conn *mdns.Conn, cancel context.CancelFunc) error {
	// Wait for SIGTERM and SIGKILL signals
	// TODO: Should replace that with a context passed as a parameter with the signal handling done upstream.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- process.Wait()
	}()

	var err error
	// Wait for the signals or for the child process to stop
	log.Info("Waiting for the kubelet to stop or receive a stop signal...")
	for alive := true; alive; {
		select {
		case <-stop:
			// Stop the cmd process
			log.Info("Received TERM Signal. Stopping kubelet...")
			err = process.Signal(syscall.SIGTERM)
			if err == nil {
				err = process.Wait()
				if err != nil {
					log.WithError(err).Warn("Error while waiting for kubelet to stop")
				}
			}

			alive = false
		case <-cmdDone:
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

	if err == nil && cancel != nil {
		log.Info("Canceling the context...")
		cancel()
	}

	return fmt.Errorf("failed to wait for kubelet: %w", err)
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
	_, cancel := data.ContextWithCancel()

	err := WaitForKubelet(kubeletProcess, conn, cancel)

	data.IkniteCluster().Update(iknite.Stopping, "stop", nil, nil)
	if err == nil {
		// Prevent double stop
		data.SetKubeletProcess(nil)
	}

	ensureServerStopped(data)

	data.IkniteCluster().Update(iknite.Cleaning, "clean", nil, nil)
	err = k8s.CleanAll(data.Host(), &data.IkniteCluster().Spec, true, false, false, false)
	if err != nil {
		log.WithError(err).Warn("Error during cleanup after kubelet stopped")
	}
	data.IkniteCluster().Update(iknite.Stopped, "", nil, nil)
	return nil
}
