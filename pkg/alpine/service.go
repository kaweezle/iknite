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

import (
	"os"
	"os/exec"
	"path"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	openRCDirectory = "/run/openrc"
	servicesDir     = "/etc/init.d"
	runLevelDir     = "/etc/runlevels/default"
)

var startedServicesDir = path.Join(openRCDirectory, "started")

func StartOpenRC() (err error) {
	err = utils.ExecuteIfNotExist(openRCDirectory, func() error {
		log.Info("Starting openrc...")
		if out, err := exec.Command("/sbin/openrc", "default").CombinedOutput(); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return errors.Wrap(err, "Error while starting openrc")
		}
	})

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
		if out, err := exec.Command("/sbin/rc-service", serviceName, "stop").Output(); err == nil {
			log.Trace(string(out))
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
