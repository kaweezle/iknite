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

// cSpell:words klog cleanupnode
// cSpell:disable
import (
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"

	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

// cSpell:enable

// NewCleanupNodePhase creates a kubeadm workflow phase that cleanup the node
func NewCleanupNodePhase() workflow.Phase {
	return workflow.Phase{
		Name:    "cleanup-node",
		Aliases: []string{"cleanupnode"},
		Short:   "Run cleanup node.",
		Run:     runCleanupNode,
		InheritFlags: []string{
			options.CertificatesDir,
			options.NodeCRISocket,
			options.CleanupTmpDir,
			options.DryRun,
		},
	}
}

func runCleanupNode(c workflow.RunData) error {
	r, ok := c.(IkniteResetData)
	if !ok {
		return errors.New("cleanup-node phase invoked with an invalid data struct")
	}

	return k8s.CleanAll(&r.IkniteCluster().Spec, true, true, r.DryRun())

}
