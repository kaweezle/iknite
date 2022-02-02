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

func EnsureNetFilter() (err error) {
	err = utils.ExecuteIfNotExist(conntrackFile, func() error {
		log.Info("Enabling netfilter...")
		if out, err := exec.Command("/sbin/modprobe", netfilter_module).CombinedOutput(); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return errors.Wrap(err, "Error while starting openrc")
		}
	})

	return
}
