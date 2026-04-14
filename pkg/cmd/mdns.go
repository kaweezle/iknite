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
	"fmt"
	"net"

	"github.com/pion/mdns"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/ipv4"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
)

func NewMdnsCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	// configureCmd represents the start command
	mdnsCmd := &cobra.Command{
		Use:   "mdns",
		Short: "Publish cluster hostname through mdns",
		Long: `On WSL, publishing the localhost over mdns allows avoiding messing
with the DNS on the Windows side.

It assumes that mDNS is not use elsewhere inside WSL.
`,
		Run: func(_ *cobra.Command, _ []string) {
			performMdns(ikniteConfig)
		},
	}

	config.AddIkniteClusterFlags(mdnsCmd.Flags(), ikniteConfig)

	return mdnsCmd
}

func performMdns(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	addr, err := net.ResolveUDPAddr("udp", mdns.DefaultAddress)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("cannot resolve default address: %w", err))
	}

	l, err := net.ListenUDP("udp4", addr)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("cannot listen on default address: %w", err))
	}

	logrus.WithFields(logrus.Fields{
		"domainName": ikniteConfig.DomainName,
		"addr":       addr,
	}).Debug("Start mdns responder...")

	_, err = mdns.Server(ipv4.NewPacketConn(l), &mdns.Config{
		LocalNames: []string{ikniteConfig.DomainName},
	})
	if err != nil {
		cobra.CheckErr(fmt.Errorf("cannot create server: %w", err))
	}
	select {}
}
