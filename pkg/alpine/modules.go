package alpine

// cSpell: words netfilter conntrack
// cSpell: disable
import (
	"fmt"
	"os"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: enable

const (
	netfilter_module = "br_netfilter"
	brNetfilterDir   = "/proc/sys/net/bridge"
	machineIDFile    = "/etc/machine-id"
	modProbeCmd      = "/sbin/modprobe"
)

// EnsureNetFilter ensures net filtering is available. It does so by checking
// The availability of the /proc/sys/net/bridge directory. On Windows 11, WSL2
// includes br_netfilter in the kernel and modprobe is not available.
// On other linuxes, netfilter is provided as a module.
func EnsureNetFilter(fsExe host.FileExecutor) error {
	if err := host.ExecuteIfNotExist(fsExe, brNetfilterDir, func() error {
		log.Debug("Enabling netfilter...")
		if out, err := fsExe.Run(true, modProbeCmd, netfilter_module); err == nil {
			log.Trace(string(out))
		} else {
			return fmt.Errorf("error while enabling netfilter: %s: %w", string(out), err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to ensure netfilter: %w", err)
	}
	return nil
}

func EnsureMachineID(fs host.FileSystem) error {
	if err := host.ExecuteIfNotExist(fs, machineIDFile, func() error {
		id := uuid.New()
		log.WithFields(log.Fields{
			"uuid":     id,
			"filename": machineIDFile,
		}).Info("Generating machine ID...")

		if err := fs.WriteFile(
			machineIDFile,
			[]byte(id.String()),
			os.FileMode(int(0o644)),
		); err != nil {
			return fmt.Errorf("error while creating machine id: %s: %w", machineIDFile, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to ensure machine ID: %w", err)
	}
	return nil
}
