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
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: enable

const (
	openRCSourceDirectory = "/lib/rc/init.d"
	openRCDirectory       = "/run/openrc"
	servicesDir           = "/etc/init.d"
	runLevelDir           = "/etc/runlevels/default"
)

var startedServicesDir = path.Join(openRCDirectory, "started")

func EnsureOpenRC(h host.Executor, level string) error {
	log.WithField("level", level).Info("Ensuring OpenRC...")
	if out, err := h.Run(true, "/sbin/openrc", "default"); err == nil {
		log.Trace(string(out))
		return nil
	} else {
		return fmt.Errorf("error while starting openrc: %w", err)
	}
}

// StartOpenRC starts the openrc services in the default runlevel.
// If one of the services is already started, it is not restarted. It one is
// not started, it is started.
func StartOpenRC(h host.FileExecutor) error {
	if err := host.ExecuteIfNotExist(h, constants.SoftLevelPath, func() error {
		return EnsureOpenRC(h, "default")
	}); err != nil {
		return fmt.Errorf("failed to start OpenRC: %w", err)
	}
	return nil
}

func IsServiceStarted(h host.FileSystem, serviceName string) (bool, error) {
	serviceLink := path.Join(startedServicesDir, serviceName)
	exists, err := h.Exists(serviceLink)
	if err != nil {
		return false, fmt.Errorf("failed to check if service %s is started: %w", serviceName, err)
	}
	return exists, nil
}

// ExecuteIfServiceNotStarted executes the function fn if the service serviceName
// is not started.
func ExecuteIfServiceNotStarted(h host.FileSystem, serviceName string, fn func() error) error {
	exists, err := IsServiceStarted(h, serviceName)
	if err != nil {
		return fmt.Errorf("error while checking if service %s exists: %w", serviceName, err)
	}
	if !exists {
		return fn()
	}

	return nil
}

// ExecuteIfServiceStarted executes the fn function if the service serviceName
// is started.
func ExecuteIfServiceStarted(h host.FileSystem, serviceName string, fn func() error) error {
	exists, err := IsServiceStarted(h, serviceName)
	if err != nil {
		return fmt.Errorf("error while checking if service %s exists: %w", serviceName, err)
	}
	if exists {
		return fn()
	}

	return nil
}

// EnableService enables the service named serviceName.
func EnableService(h host.FileSystem, serviceName string) error {
	serviceFilename := path.Join(servicesDir, serviceName)
	destinationFilename := path.Join(runLevelDir, serviceName)
	if err := host.ExecuteIfNotExist(h, destinationFilename, func() error {
		return h.Symlink(serviceFilename, destinationFilename)
	}); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}
	return nil
}

// DisableService disables the service named serviceName.
func DisableService(h host.FileSystem, serviceName string) error {
	destinationFilename := path.Join(runLevelDir, serviceName)
	if err := host.ExecuteIfExist(h, destinationFilename, func() error {
		return h.Remove(destinationFilename)
	}); err != nil {
		return fmt.Errorf("failed to disable service %s: %w", serviceName, err)
	}
	return nil
}

// StartService start the serviceName service if it is not already started.
func StartService(h host.FileExecutor, serviceName string) error {
	return ExecuteIfServiceNotStarted(h, serviceName, func() error {
		if out, err := h.Run(false, "/sbin/rc-service", serviceName, "start"); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return fmt.Errorf("error while starting service %s: %w", serviceName, err)
		}
	})
}

// StopService stops the serviceName service if it is  started.
func StopService(h host.FileExecutor, serviceName string) error {
	return ExecuteIfServiceStarted(h, serviceName, func() error {
		if out, err := h.Run(false, "/sbin/rc-service", serviceName, "stop"); err == nil {
			log.Trace(string(out))
			return nil
		} else {
			return fmt.Errorf("error while stopping service %s: %w", serviceName, err)
		}
	})
}

func PretendServiceStarted(h host.FileSystem, serviceName string) error {
	networkSource := path.Join(servicesDir, serviceName)
	networkDestination := path.Join(startedServicesDir, serviceName)
	if err := host.ExecuteIfNotExist(h, networkDestination, func() error {
		return h.Symlink(networkSource, networkDestination)
	}); err != nil {
		return fmt.Errorf("failed to pretend service %s is started: %w", serviceName, err)
	}
	return nil
}

func EnsureOpenRCDirectory(h host.FileSystem) error {
	if err := host.ExecuteIfNotExist(h, openRCDirectory, func() error {
		return h.Symlink(openRCSourceDirectory, openRCDirectory)
	}); err != nil {
		return fmt.Errorf("failed to ensure OpenRC directory: %w", err)
	}
	return nil
}
