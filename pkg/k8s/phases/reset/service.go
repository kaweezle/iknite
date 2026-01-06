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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	initSystem "k8s.io/kubernetes/cmd/kubeadm/app/util/initsystem"
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

func runCleanupService(c workflow.RunData) error {
	r, ok := c.(IkniteResetData)
	if !ok {
		return errors.New("cleanup-node phase invoked with an invalid data struct")
	}

	// Try to stop the kubelet service
	klog.V(1).Infoln("[reset] Getting init system")
	initSystem, err := initSystem.GetInitSystem()
	if err != nil {
		klog.Warningln(
			"[reset] The iknite service could not be stopped by kubeadm. Unable to detect a supported init system!",
		) //nolint:lll
		klog.Warningln("[reset] Please ensure iknite is stopped manually")
	} else {
		if !r.DryRun() {
			logrus.WithField("phase", "reset").Info("Stopping the iknite service...")
			if err := initSystem.ServiceStop("iknite"); err != nil {
				klog.Warningf("[reset] The iknite service could not be stopped by kubeadm: [%v]\n", err)
				klog.Warningln("[reset] Please ensure iknite is stopped manually")
			}
		} else {
			logrus.WithField("phase", "reset").Info("Would stop the iknite service")
		}
	}

	return nil
}
