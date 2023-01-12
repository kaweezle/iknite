package alpine

import (
	"fmt"
	"net"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
)

func CheckIpExists(ip net.IP) (result bool, err error) {
	result = false
	var ifaces []net.Interface
	ifaces, err = net.Interfaces()

	if err != nil {
		return
	}
	for _, i := range ifaces {
		var addrs []net.Addr
		addrs, err = i.Addrs()
		if err != nil {
			log.WithFields(log.Fields{
				"interface": i,
			}).Warn("Cannot get interface address")
			continue
		}
		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				if v.IP.Equal(ip) {
					result = true
					return
				}
			}
		}
	}
	return
}

// AddIpAddress adds the IP address address to the interface iface.
//
// It uses the default mask of the IP address class as the mask, and the default
// broadcast address as the broadcast address.
func AddIpAddress(iface string, address net.IP) (err error) {

	ones, _ := address.DefaultMask().Size()
	ipWithMask := fmt.Sprintf("%v/%d", address, ones)

	log.WithFields(log.Fields{
		"ip":    ipWithMask,
		"iface": iface,
	}).Info("Adding IP address")

	parameters := []string{
		"addr",
		"add", ipWithMask,
		"broadcast", "+", // This will set the broadcast address automatically
		"dev", iface,
	}

	var out []byte
	if out, err = utils.Exec.Run(true, "/sbin/ip", parameters...); err != nil {
		err = errors.Wrap(err, string(out))
	}
	return
}

func removeIpAddresses(hosts *txeh.Hosts, toRemove []net.IP) {

	if len(toRemove) > 0 {
		ips := make([]string, len(toRemove))
		for i, toRem := range toRemove {
			ips[i] = toRem.String()
		}
		hosts.RemoveAddresses(ips)
	}
}

func AddIpMapping(hostConfig *txeh.HostsConfig, ip string, domainName string, toRemove []net.IP) (err error) {
	var hosts *txeh.Hosts
	hosts, err = txeh.NewHosts(hostConfig)
	if err != nil {
		return err
	}
	removeIpAddresses(hosts, toRemove)

	hosts.AddHost(ip, domainName)
	err = hosts.Save()
	return
}
