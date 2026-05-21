package mdns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type MDNSServer interface {
	QueryAddr(context.Context, string) (dnsmessage.ResourceHeader, netip.Addr, error)
	Close() error
}

func CreateMDNSConnection(cfg *mdns.Config, logger *slog.Logger) (MDNSServer, error) {
	addr4, err := net.ResolveUDPAddr("udp", mdns.DefaultAddressIPv4)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, fmt.Errorf("cannot resolve default address: %w", err)
	}

	addr6, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, fmt.Errorf("cannot resolve default address: %w", err)
	}

	l4, err := net.ListenUDP("udp4", addr4)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, fmt.Errorf("cannot listen on default address: %w", err)
	}

	l6, err := net.ListenUDP("udp6", addr6)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, fmt.Errorf("cannot listen on default address: %w", err)
	}

	conn, err := mdns.Server(ipv4.NewPacketConn(l4), ipv6.NewPacketConn(l6), cfg)
	if err != nil { // nocov -- should not happen on supported platforms
		_ = l4.Close() //nolint:errcheck // best effort cleanup
		_ = l6.Close() //nolint:errcheck // best effort cleanup
		return nil, fmt.Errorf("cannot create server: %w", err)
	}
	logger.Debug("Start mdns responder...", "addr4", addr4, "addr6", addr6, "interface4", l4.LocalAddr(),
		"interface6", l6.LocalAddr())

	return conn, nil
}
