package alpine

import (
	"os/exec"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	netfilter_module = "br_netfilter"
	conntrackFile    = "/proc/sys/net/nf_conntrack_max"
)

func EnsureNetFilter() error {
	return utils.ExecuteIfExist("/lib/modules", func() (err error) {
		log.Debug("Enabling netfilter...")
		var out []byte
		if out, err = exec.Command("/sbin/modprobe", netfilter_module).CombinedOutput(); err == nil {
			log.Trace(string(out))
		} else {
			err = errors.Wrapf(err, "Error while enabling netfilter: %s", string(out))
		}
		return
	})
}
