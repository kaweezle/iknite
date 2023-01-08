package alpine

import (
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	netfilter_module = "br_netfilter"
	conntrackFile    = "/proc/sys/net/nf_conntrack_max"
	brNetfilterDir   = "/proc/sys/net/bridge"
)

// EnsureNetFilter ensures net filtering is available. It does so by checking
// The availability of the /proc/sys/net/bridge directory. On Windows 11, WSL2
// includes br_netfilter in the kernel and modprobe is not available.
// On other linuxes, netfilter is provided as a module.
func EnsureNetFilter() error {
	return utils.ExecuteIfNotExist(brNetfilterDir, func() (err error) {
		log.Debug("Enabling netfilter...")
		var out []byte
		if out, err = utils.Exec.Run(true, "/sbin/modprobe", netfilter_module); err == nil {
			log.Trace(string(out))
		} else {
			err = errors.Wrapf(err, "Error while enabling netfilter: %s", string(out))
		}
		return
	})
}
