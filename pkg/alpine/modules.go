package alpine

// cSpell: words netfilter conntrack
// cSpell: disable
import (
	"os"

	"github.com/google/uuid"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// cSpell: enable

const (
	netfilter_module = "br_netfilter"
	brNetfilterDir   = "/proc/sys/net/bridge"
	machineIDFile    = "/etc/machine-id"
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

func EnsureMachineID() error {
	return utils.ExecuteIfNotExist(machineIDFile, func() (err error) {
		id := uuid.New()
		log.WithFields(log.Fields{
			"uuid":     id,
			"filename": machineIDFile,
		}).Info("Generating machine ID...")

		if err = utils.WriteFile(machineIDFile, []byte(id.String()), os.FileMode(int(0644))); err != nil {
			err = errors.Wrapf(err, "Error while creating machine id: %s", machineIDFile)
		}
		return
	})
}
