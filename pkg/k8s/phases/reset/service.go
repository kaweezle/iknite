/*
Copyright 2019 The Kubernetes Authors.

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

package reset

// cSpell:words klog cleanupservice
// cSpell:disable
import (
	"errors"

	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell:enable

// NewCleanupNodePhase creates a kubeadm workflow phase that cleanup the node.
func NewCleanupServicePhase() workflow.Phase {
	return workflow.Phase{
		Name:    "cleanup-service",
		Aliases: []string{"cleanupservice"},
		Short:   "Run cleanup service.",
		Run:     runCleanupService,
		InheritFlags: []string{
			options.CertificatesDir,
			options.NodeCRISocket,
			options.CleanupTmpDir,
			options.DryRun,
		},
	}
}

type cleanupServiceData interface {
	utils.LoggerProvider
	host.HostProvider
	DryRun() bool
}

func runCleanupService(c workflow.RunData) error {
	r, ok := c.(cleanupServiceData)
	if !ok {
		return errors.New("cleanup-node phase invoked with an invalid data struct")
	}

	logger := r.Logger().With("phase", "reset", "service", constants.IkniteService)
	alpineHost := r.Host()

	// Try to stop the kubelet service
	logger.Info("Getting the init system...")

	if !r.DryRun() {
		logger.Info("Stopping the service")
		err := alpine.StopService(alpineHost, constants.IkniteService, logger)
		if err != nil {
			klog.Warningf("[reset] The iknite service could not be stopped: [%v]\n", err)
			klog.Warningln("[reset] Please ensure iknite is stopped manually")
		}
	} else {
		logger.Info("Would stop the service")
	}

	return nil
}
