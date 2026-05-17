package init

// cSpell: disable
import (
	"fmt"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
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
	IkniteClusterListenerRegistrar
	ShutdownHookRegistrar
	utils.LoggerProvider
}

func runServe(c workflow.RunData) error {
	data, ok := c.(serverData)
	if !ok {
		return fmt.Errorf("serve phase invoked with an invalid data struct")
	}

	ikniteCluster := data.IkniteCluster()
	srv, err := server.StartIkniteServer(data.Host(), constants.KubernetesPKIDir, ikniteCluster, data.Logger())
	if err != nil {
		return fmt.Errorf("failed to start iknite status server: %w", err)
	}

	data.Logger().Info("Iknite status server started", "port", ikniteCluster.Spec.StatusServerPort)

	ch, unregister := data.RegisterIkniteClusterListener()
	go func() {
		for cluster := range ch {
			srv.SetCluster(cluster)
		}
	}()
	data.RegisterShutdownHook("serve", func() error {
		unregister()
		return srv.Shutdown()
	})
	return nil
}
