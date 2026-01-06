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
package alpine

// cSpell: words runlevel runlevels softlevel
// cSpell: disable
import (
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

const (
	openRCSourceDirectory = "/lib/rc/init.d"
	openRCDirectory       = "/run/openrc"
	servicesDir           = "/etc/init.d"
	runLevelDir           = "/etc/runlevels/default"
)

var startedServicesDir = path.Join(openRCDirectory, "started")

func EnsureOpenRC(level string) error {
	log.WithField("level", level).Info("Ensuring OpenRC...")
	if out, err := utils.Exec.Run(true, "/sbin/openrc", "default"); err == nil {
		log.Trace(string(out))
		return nil
	} else {
		return errors.Wrap(err, "Error while starting openrc")
	}
}

// StartOpenRC starts the openrc services in the default runlevel.
// If one of the services is already started, it is not restarted. It one is
// not started, it is started.
func StartOpenRC() (err error) {
	if err := utils.ExecuteIfNotExist(constants.SoftLevelPath, func() error {
		return EnsureOpenRC("default")
	}); err != nil {
		return fmt.Errorf("failed to start OpenRC: %w", err)
	}
	return nil
}

func IsServiceStarted(serviceName string) (bool, error) {
	serviceLink := path.Join(startedServicesDir, serviceName)
	exists, err := utils.Exists(serviceLink)
	if err != nil {
		return false, fmt.Errorf("failed to check if service %s is started: %w", serviceName, err)
	}
	return exists, nil
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

// ExecuteIfServiceStarted executes the fn function if the service serviceName
// is started.
func ExecuteIfServiceStarted(serviceName string, fn func() error) error {
	exists, err := IsServiceStarted(serviceName)
	if err != nil {
		return errors.Wrapf(err, "Error while checking if service %s exists", serviceName)
	}
	if exists {
		return fn()
	}

	return nil
}

// EnableService enables the service named serviceName.
func EnableService(serviceName string) error {
	serviceFilename := path.Join(servicesDir, serviceName)
	destinationFilename := path.Join(runLevelDir, serviceName)
	if err := utils.ExecuteIfNotExist(destinationFilename, func() error {
		return os.Symlink(serviceFilename, destinationFilename)
	}); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}
	return nil
}

// DisableService disables the service named serviceName.
func DisableService(serviceName string) error {
	destinationFilename := path.Join(runLevelDir, serviceName)
	if err := utils.ExecuteIfExist(destinationFilename, func() error {
		return os.Remove(destinationFilename)
	}); err != nil {
		return fmt.Errorf("failed to disable service %s: %w", serviceName, err)
	}
	return nil
}

// StartService start the serviceName service if it is not already started.
func StartService(serviceName string) error {
	return ExecuteIfServiceNotStarted(serviceName, func() error {
		if out, err := utils.Exec.Run(false, "/sbin/rc-service", serviceName, "start"); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return errors.Wrapf(err, "Error while starting service %s", serviceName)
		}
	})
}

// StopService stops the serviceName service if it is  started.
func StopService(serviceName string) error {
	return ExecuteIfServiceStarted(serviceName, func() error {
		if out, err := utils.Exec.Run(false, "/sbin/rc-service", serviceName, "stop"); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return errors.Wrapf(err, "Error while starting service %s", serviceName)
		}
	})
}

func PretendServiceStarted(serviceName string) error {
	networkSource := path.Join(servicesDir, serviceName)
	networkDestination := path.Join(startedServicesDir, serviceName)
	if err := utils.ExecuteIfNotExist(networkDestination, func() error {
		return os.Symlink(networkSource, networkDestination)
	}); err != nil {
		return fmt.Errorf("failed to pretend service %s is started: %w", serviceName, err)
	}
	return nil
}

func EnsureOpenRCDirectory() error {
	if err := utils.ExecuteIfNotExist(openRCDirectory, func() error {
		return os.Symlink(openRCSourceDirectory, openRCDirectory)
	}); err != nil {
		return fmt.Errorf("failed to ensure OpenRC directory: %w", err)
	}
	return nil
}
