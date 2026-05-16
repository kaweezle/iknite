/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// cSpell: words sirupsen
package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"

	"github.com/pion/mdns/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/config"
)

type mdnsServer interface {
	QueryAddr(context.Context, string) (dnsmessage.ResourceHeader, netip.Addr, error)
	Close() error
}

var newMdnsServerFn = func(cfg *mdns.Config) (mdnsServer, net.Addr, net.Addr, error) {
	addr4, err := net.ResolveUDPAddr("udp", mdns.DefaultAddressIPv4)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, nil, nil, fmt.Errorf("cannot resolve default address: %w", err)
	}

	l4, err := net.ListenUDP("udp4", addr4)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, nil, nil, fmt.Errorf("cannot listen on default address: %w", err)
	}

	addr6, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, nil, nil, fmt.Errorf("cannot resolve default address: %w", err)
	}

	l6, err := net.ListenUDP("udp6", addr6)
	if err != nil { // nocov -- should not happen on supported platforms
		return nil, nil, nil, fmt.Errorf("cannot listen on default address: %w", err)
	}

	conn, err := mdns.Server(ipv4.NewPacketConn(l4), ipv6.NewPacketConn(l6), cfg)
	if err != nil {
		_ = l4.Close() //nolint:errcheck // best effort cleanup
		_ = l6.Close() //nolint:errcheck // best effort cleanup
		return nil, nil, nil, fmt.Errorf("cannot create server: %w", err)
	}

	return conn, addr4, addr6, nil
}

func NewMdnsCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	// configureCmd represents the start command
	mdnsCmd := &cobra.Command{
		Use:   "mdns",
		Short: "Publish cluster hostname through mdns",
		Long: `On WSL, publishing the localhost over mdns allows avoiding messing
with the DNS on the Windows side.

It assumes that mDNS is not use elsewhere inside WSL.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return performMdns(cmd.Context(), ikniteConfig)
		},
	}

	config.AddIkniteClusterFlags(mdnsCmd.PersistentFlags(), ikniteConfig)
	mdnsCmd.AddCommand(NewMdnsTestCmd(ikniteConfig))

	return mdnsCmd
}

func NewMdnsTestCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Query the cluster hostname through mdns",
		Long: `Queries the configured cluster domain using mDNS and prints the response.

It starts the same mDNS client/server transport used by the mdns command and
queries the configured domain name once.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return performMdnsTest(cmd.Context(), cmd.OutOrStdout(), ikniteConfig)
		},
	}
}

func performMdns(ctx context.Context, ikniteConfig *v1alpha1.IkniteClusterSpec) error {
	conn, addr4, addr6, err := newMdnsServerFn(&mdns.Config{
		LocalNames: []string{ikniteConfig.DomainName},
	})
	if err != nil {
		return err
	}

	logger := util.GetLoggerFromContext(ctx)
	logger.WithFields(logrus.Fields{
		"domainName": ikniteConfig.DomainName,
		"addr4":      addr4,
		"addr6":      addr6,
	}).Debug("Start mdns responder...")

	defer conn.Close() //nolint:errcheck // should not fail.
	<-ctx.Done()
	logger.Info("Shutting down mdns responder...")
	return nil
}

func performMdnsTest(ctx context.Context, out io.Writer, ikniteConfig *v1alpha1.IkniteClusterSpec) error {
	conn, _, _, err := newMdnsServerFn(&mdns.Config{})
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // should not fail.

	answer, src, queryErr := conn.QueryAddr(ctx, ikniteConfig.DomainName)
	if queryErr != nil {
		return fmt.Errorf("cannot query domain %q: %w", ikniteConfig.DomainName, queryErr)
	}
	if _, writeErr := fmt.Fprintln(out, answer); writeErr != nil { // nocov -- unlikely to fail
		return fmt.Errorf("cannot write mdns answer: %w", writeErr)
	}
	if _, writeErr := fmt.Fprintln(out, src); writeErr != nil { // nocov -- unlikely to fail
		return fmt.Errorf("cannot write mdns source: %w", writeErr)
	}

	return nil
}
