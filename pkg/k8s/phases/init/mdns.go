package init

// cSpell: disable
import (
	"fmt"

	mdnsLib "github.com/pion/mdns/v2"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/mdns"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func NewMDnsPublishPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "mdns-publish",
		Short: "Publish the cluster domain with mdns.",
		Run:   runMDnsPublish,
	}
}

type mdnsData interface {
	IkniteClusterProvider
	ShutdownHookRegistrar
	utils.LoggerProvider
}

// runMDnsPublish executes the mdns publish phase.
func runMDnsPublish(c workflow.RunData) error {
	data, ok := c.(mdnsData)
	if !ok {
		return fmt.Errorf("mdns phase invoked with an invalid data struct. ")
	}
	logger := data.Logger().With("phase", "mdns-publish")
	ikniteConfig := data.IkniteCluster().Spec

	if !ikniteConfig.EnableMDNS {
		logger.Info("MDNS is disabled, skipping mdns publish phase.")
		return nil
	}
	conn, err := mdns.CreateMDNSConnection(&mdnsLib.Config{
		LocalNames: []string{ikniteConfig.DomainName},
	}, logger)
	if err != nil { // nocov -- hard to test mdns server creation failure
		return fmt.Errorf("cannot create mdns server: %w", err)
	}

	data.RegisterShutdownHook("mdns", conn.Close)

	return nil
}
