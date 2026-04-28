package alpine

// cSpell: words iface ifaces
// cSpell: disable
import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/txn2/txeh"

	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: enable

// AddIpAddress adds the IP address to the interface iface.
//
// It uses the default mask of the IP address class as the mask, and the default
// broadcast address as the broadcast address.
func AddIpAddress(exec host.Executor, iface string, address net.IP) error {
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
	if out, err := exec.Run(true, "/sbin/ip", parameters...); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
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

func IpMappingForHost(hosts *txeh.Hosts, domainName string) (net.IP, error) {
	found, address, _ := hosts.HostAddressLookup(domainName, txeh.IPFamilyV4)
	if !found {
		return nil, fmt.Errorf("no IP address found for %s", domainName)
	} else {
		return net.ParseIP(address), nil
	}
}

func AddIpMapping(
	hostsConfig *txeh.HostsConfig,
	ip net.IP,
	domainName string,
	toRemove []net.IP,
) error {
	hosts, err := txeh.NewHosts(hostsConfig)
	if err != nil {
		return fmt.Errorf("failed to create hosts file handler: %w", err)
	}
	removeIpAddresses(hosts, toRemove)

	hosts.AddHost(ip.String(), domainName)
	err = hosts.Save()
	if err != nil {
		return fmt.Errorf("failed to save hosts file: %w", err)
	}
	return nil
}
