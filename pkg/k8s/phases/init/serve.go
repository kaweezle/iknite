package init

// cSpell: disable
import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/server"
)

// cSpell: enable

func NewServePhase() workflow.Phase {
	return workflow.Phase{
		Name:  "serve",
		Short: "Start the iknite status HTTPS server.",
		Run:   runServe,
	}
}

func runServe(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("serve phase invoked with an invalid data struct")
	}

	ikniteSpec := data.IkniteCluster().Spec
	srv, err := server.StartIkniteServer(constants.KubernetesPKIDir, ikniteSpec.Ip, constants.IkniteServerPort)
	if err != nil {
		return fmt.Errorf("failed to start iknite status server: %w", err)
	}

	log.WithField("port", constants.IkniteServerPort).Info("Iknite status server started")
	data.SetStatusServer(srv)
	return nil
}

// ensureServerStopped stops the status server if it is still running.
// It is called from the daemonize phase after the kubelet has stopped.
func ensureServerStopped(data IkniteInitData) {
	srv := data.StatusServer()
	if srv == nil {
		return
	}
	if err := server.ShutdownServer(srv); err != nil {
		log.WithError(err).Warn("Error stopping iknite status server")
	}
	data.SetStatusServer(nil)
}
