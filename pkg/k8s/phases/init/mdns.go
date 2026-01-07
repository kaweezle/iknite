package init

// cSpell: disable
import (
	"fmt"
	"net"

	"github.com/pion/mdns"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

// cSpell: enable

func NewMDnsPublishPhase() workflow.Phase {
	return workflow.Phase{
		Name:  "mdns-publish",
		Short: "Publish the cluster domain with mdns.",
		Run:   runMDnsPublish,
	}
}

// runPrepare executes the node initialization process.
func runMDnsPublish(c workflow.RunData) error {
	data, ok := c.(IkniteInitData)
	if !ok {
		return fmt.Errorf("prepare phase invoked with an invalid data struct. ")
	}
	ikniteConfig := data.IkniteCluster().Spec

	if !ikniteConfig.EnableMDNS {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", mdns.DefaultAddress)
	if err != nil {
		return errors.Wrap(err, "Cannot resolve default address")
	}

	l, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return errors.Wrap(err, "Cannot Listen on default address")
	}

	var conn *mdns.Conn
	log.WithField("phase", "mdns-publish").Info("Starting the mdns responder...")
	conn, err = mdns.Server(ipv4.NewPacketConn(l), &mdns.Config{
		LocalNames: []string{ikniteConfig.DomainName},
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create server")
	}
	data.SetMDnsConn(conn)

	return nil
}
