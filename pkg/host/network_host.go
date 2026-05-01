// cSpell: words iface ifaces sirupsen
package host

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
)

type NetworkHost interface {
	GetOutboundIP( /* ctx context.Context*/ ) (net.IP, error)
	CheckIpExists(ip net.IP) (bool, error)
	GetHostsConfig() *txeh.HostsConfig
	IsHostMapped(ctx context.Context, ip net.IP, domainName string) (bool, []net.IP)
}

type NetworkHostImpl struct{}

var _ NetworkHost = (*hostImpl)(nil)

func NewDefaultNetworkHost() NetworkHost {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}

func (h *hostImpl) GetOutboundIP( /* ctx context.Context */ ) (net.IP, error) {
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "udp", "8.8.8.8:80")
	if err != nil { // nocov - This is a fallback for environments without network access, which is hard to test in CI
		return nil, fmt.Errorf("error while getting IP address: %w", err)
	}
	defer func() {
		err = conn.Close()
	}()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok { // nocov - This is a fallback for environments without network access, which is hard to test in CI
		return nil, fmt.Errorf("failed to get local address")
	}

	return localAddr.IP.To16(), nil
}

func (h *hostImpl) CheckIpExists(ip net.IP) (bool, error) {
	result := false
	ifaces, err := net.Interfaces()
	if err != nil { // nocov - This is a fallback for environments without network access, which is hard to test in CI
		return result, fmt.Errorf("failed to get network interfaces: %w", err)
	}
	for _, i := range ifaces {
		var addrs []net.Addr
		addrs, err = i.Addrs()
		if err != nil { // nocov - Hard in CI
			logrus.WithFields(logrus.Fields{
				"interface": i,
			}).Warn("Cannot get interface address")
			continue
		}
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				if ipNet.IP.Equal(ip) {
					result = true
					return result, nil
				}
			}
		}
	}
	return result, nil
}

func (h *hostImpl) GetHostsConfig() *txeh.HostsConfig {
	return &txeh.HostsConfig{}
}

func (h *hostImpl) IsHostMapped(ctx context.Context, ip net.IP, domainName string) (bool, []net.IP) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domainName)
	contains := false
	if err != nil {
		ips = []net.IP{}
	} else {
		for _, existing := range ips {
			if existing.Equal(ip) {
				contains = true
				break
			}
		}
	}
	return contains, ips
}
