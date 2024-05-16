package phases

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pion/mdns"
	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

func NewDaemonizePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "daemonize",
		Short: "Wait for the kubelet to stop.",
		Run:   runDaemonize,
	}
}

func WaitForKubelet(cmd *exec.Cmd, conn *mdns.Conn) error {

	// Wait for SIGTERM and SIGKILL signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	var err error
	// Wait for the signals or for the child process to stop
	log.Info("Waiting for the kubelet to stop or receive a stop signal...")
	for alive := true; alive; {
		select {
		case <-stop:
			// Stop the cmd process
			log.Info("Recevied TERM Signal. Stopping kubelet...")
			err = cmd.Process.Signal(syscall.SIGTERM)
			if err == nil {
				cmd.Wait()
			}

			alive = false
		case <-cmdDone:
			// Child process has stopped
			log.Infof("Kubelet stopped with state: %s", cmd.ProcessState.String())
			alive = false
		}

	}

	if err == nil && conn != nil {
		log.Info("Stopping the mdns responder...")
		err = conn.Close()
	}

	return err
}

// runPrepare executes the node initialization process.
func runDaemonize(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	cmd := data.KubeletCmd()
	if cmd == nil {
		return nil
	}
	conn := data.MDnsConn()

	err := WaitForKubelet(cmd, conn)
	if err == nil {
		// Prevent double stop
		data.SetKubeletCmd(nil)
	}
	k8s.CleanAll(data.IkniteConfig())
	return err
}
