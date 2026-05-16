package init

// cSpell: disable
import (
	"fmt"
	"net"

	"github.com/pion/mdns/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

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

// runPrepare executes the node initialization process.
func runMDnsPublish(c workflow.RunData) error {
	data, ok := c.(mdnsData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	ikniteConfig := data.IkniteCluster().Spec
	logger := data.Logger()

	if !ikniteConfig.EnableMDNS {
		logger.WithField("phase", "mdns-publish").Info("MDNS is disabled, skipping mdns publish phase.")
		return nil
	}

	addr4, err := net.ResolveUDPAddr("udp", mdns.DefaultAddressIPv4)
	if err != nil {
		return fmt.Errorf("cannot resolve default address: %w", err)
	}

	l4, err := net.ListenUDP("udp4", addr4)
	if err != nil {
		return fmt.Errorf("cannot listen on default address: %w", err)
	}

	addr6, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6)
	if err != nil {
		return fmt.Errorf("cannot resolve default address: %w", err)
	}

	l6, err := net.ListenUDP("udp6", addr6)
	if err != nil {
		return fmt.Errorf("cannot listen on default address: %w", err)
	}

	logger.WithField("phase", "mdns-publish").WithFields(logrus.Fields{
		"addr4":      addr4,
		"addr6":      addr6,
		"interface4": l4.LocalAddr(),
		"interface6": l6.LocalAddr(),
	}).Debug("Start mdns responder...")

	var conn *mdns.Conn
	logger.WithField("phase", "mdns-publish").Info("Starting the mdns responder...")
	conn, err = mdns.Server(ipv4.NewPacketConn(l4), ipv6.NewPacketConn(l6), &mdns.Config{
		LocalNames: []string{ikniteConfig.DomainName},
	})
	if err != nil {
		return fmt.Errorf("cannot create server: %w", err)
	}
	data.RegisterShutdownHook("mdns", conn.Close)

	return nil
}
