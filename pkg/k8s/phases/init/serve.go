package init

// cSpell: disable
import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
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

type serverData interface {
	IkniteClusterProvider
	host.HostProvider
	StatusServerSetter
}

func runServe(c workflow.RunData) error {
	data, ok := c.(serverData)
	if !ok {
		return fmt.Errorf("serve phase invoked with an invalid data struct")
	}

	ikniteCluster := data.IkniteCluster()
	srv, err := server.StartIkniteServer(data.Host(), constants.KubernetesPKIDir, ikniteCluster)
	if err != nil {
		return fmt.Errorf("failed to start iknite status server: %w", err)
	}

	log.WithField("port", ikniteCluster.Spec.StatusServerPort).Info("Iknite status server started")
	data.SetStatusServer(srv)
	return nil
}
