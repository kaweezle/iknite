package alpine

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/antoinemartin/k8wsl/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const openRCDirectory = "/run/openrc"
const servicesDir = "/etc/init.d"
const runLevelDir = "/etc/runlevels/default"

var startedServicesDir = path.Join(openRCDirectory, "started")
var softLevelFile = path.Join(openRCDirectory, "softlevel")

func StartOpenRC() (err error) {
	err = utils.ExecuteIfNotExist(openRCDirectory, func() error {
		log.Info("Starting openrc...")
		if out, err := exec.Command("/sbin/openrc", "-n").CombinedOutput(); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return errors.Wrap(err, "Error while starting openrc")
		}
	})

	if err != nil {
		// OpenRC is picky when starting services if it hasn't been started by
		// init. In our case, init is provided by WSL. Creating this file makes
		// OpenRC happy.
		err = utils.ExecuteIfNotExist(softLevelFile, func() error {
			return utils.WriteFile(softLevelFile, []byte{}, os.FileMode(int(0444)))
		})
	}
	return
}

func IsServiceStarted(serviceName string) (bool, error) {
	serviceLink := path.Join(startedServicesDir, serviceName)
	return utils.Exists(serviceLink)
}

// ExecuteIfServiceNotStarted executes the function fn if the service serviceName
// is not started.
func ExecuteIfServiceNotStarted(serviceName string, fn func() error) error {
	exists, err := IsServiceStarted(serviceName)
	if err != nil {
		return errors.Wrapf(err, "Error while checking if service %s exists", serviceName)
	}
	if !exists {
		return fn()
	}

	return nil
}

// EnableService enables the service named serviceName
func EnableService(serviceName string) error {
	serviceFilename := path.Join(servicesDir, serviceName)
	destinationFilename := path.Join(runLevelDir, serviceName)
	return utils.ExecuteIfNotExist(destinationFilename, func() error {
		return os.Symlink(serviceFilename, destinationFilename)
	})
}

// StartService start the serviceName service if it is not already started.
func StartService(serviceName string) error {
	return ExecuteIfServiceNotStarted(serviceName, func() error {
		if out, err := exec.Command("/sbin/rc-service", serviceName, "start").Output(); err == nil {
			fmt.Println(string(out))
			return nil
		} else {
			return errors.Wrapf(err, "Error while starting service %s", serviceName)
		}
	})
}

func PretendServiceStarted(serviceName string) error {
	var networkSource = path.Join(servicesDir, serviceName)
	var networkDestination = path.Join(openRCDirectory, "started", serviceName)
	return utils.ExecuteIfNotExist(networkDestination, func() error {
		return os.Symlink(networkSource, networkDestination)
	})
}
