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
package cmd

import (
	"net"
	"os"

	"github.com/pion/mdns"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/ipv4"
)

// configureCmd represents the start command
var mdnsCmd = &cobra.Command{
	Use:   "mdns",
	Short: "Publish current hostname through mdns",
	Long: `On WSL, publishing the localhost over mdns allows avoiding messing
with the DNS on the Windows side.
`,
	PersistentPreRun: mdnsPersistentPreRun,
	Run:              performMdns,
}

func init() {
	rootCmd.AddCommand(mdnsCmd)
	hostname, err := os.Hostname()
	cobra.CheckErr(err)
	mdnsCmd.Flags().String("domain-name", hostname, "Domain name of the cluster")
}

func mdnsPersistentPreRun(cmd *cobra.Command, args []string) {
	viper.BindPFlag("cluster.domain_name", cmd.Flags().Lookup("domain-name"))
}

func performMdns(cmd *cobra.Command, args []string) {

	addr, err := net.ResolveUDPAddr("udp", mdns.DefaultAddress)
	cobra.CheckErr(errors.Wrap(err, "Cannot resolve default address"))

	l, err := net.ListenUDP("udp4", addr)
	cobra.CheckErr(errors.Wrap(err, "Cannot Listen on default address"))

	_, err = mdns.Server(ipv4.NewPacketConn(l), &mdns.Config{
		LocalNames: []string{viper.GetString("cluster.domain_name")},
	})
	cobra.CheckErr(errors.Wrap(err, "Cannot create server"))
	select {}
}
