package alpine

import (
	"os/exec"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	netfilter_module = "br_netfilter"
	conntrackFile    = "/proc/sys/net/nf_conntrack_max"
)

func EnsureNetFilter() (err error) {
	log.Debug("Enabling netfilter...")
	var out []byte
	if out, err = exec.Command("/sbin/modprobe", netfilter_module).CombinedOutput(); err == nil {
		log.Trace(string(out))
	} else {
		err = errors.Wrap(err, "Error while starting openrc")
	}

	return
}
