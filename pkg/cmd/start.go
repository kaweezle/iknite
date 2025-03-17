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
package cmd

// cSpell: disable
import (
	"context"
	"os"
	"time"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

// cSpell: enable

func NewStartCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {

	// startCmd represents the start command
	var startCmd = &cobra.Command{
		Use:   "start",
		Short: "Creates or starts the cluster",
		Long: `Starts the cluster. Performs the following operations:

- Starts OpenRC,
- Starts containerd,
- If Kubelet has never been started, execute kubeadm init to provision
  the cluster,
- Allows the use of kubectl from the root account,
- Installs flannel, metal-lb and local-path-provisioner.
`,
		PersistentPreRun: config.StartPersistentPreRun,
		Run:              func(cmd *cobra.Command, args []string) { performStart(ikniteConfig) },
	}
	flags := startCmd.Flags()

	flags.IntVarP(&timeout, options.Timeout, "t", timeout, "Wait timeout in seconds")
	config.ConfigureClusterCommand(flags, ikniteConfig)
	initializeKustomization(flags)

	return startCmd
}

func IsIkniteReady(ctx context.Context) (bool, error) {

	cluster, err := v1alpha1.LoadIkniteCluster()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	log.WithFields(log.Fields{
		"state":   cluster.Status.State.String(),
		"phase":   cluster.Status.CurrentPhase,
		"total":   cluster.Status.WorkloadsState.Count,
		"ready":   cluster.Status.WorkloadsState.ReadyCount,
		"unready": cluster.Status.WorkloadsState.UnreadyCount,
	}).Infof(
		"status=%s, phase=%s, Workloads total=%d, ready=%d, unready=%d",
		cluster.Status.State.String(),
		cluster.Status.CurrentPhase,
		cluster.Status.WorkloadsState.Count,
		cluster.Status.WorkloadsState.ReadyCount,
		cluster.Status.WorkloadsState.UnreadyCount,
	)

	if cluster.Status.State > iknite.Initializing && cluster.Status.WorkloadsState.Count > 0 {
		return true, nil
	}

	return false, nil
}

func performStart(ikniteConfig *v1alpha1.IkniteClusterSpec) {

	cobra.CheckErr(config.DecodeIkniteConfig(ikniteConfig))
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ikniteConfig))

	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	config, err := k8s.LoadFromDefault()
	if err == nil {
		if config.IsConfigServerAddress(ikniteConfig.GetApiEndPoint()) {
			log.Info("Kubeconfig already exists")
		} else {
			// If the configuration has changed, we stop and disable the kubelet
			// that may be started and clean the configuration, i.e. delete
			// certificates and manifests.
			log.Info("Kubernetes configuration has changed. Cleaning...")
			cobra.CheckErr(alpine.StopService(constants.IkniteService))
			cobra.CheckErr(k8s.CleanConfig())
		}
	} else {
		if !os.IsNotExist(err) {
			cobra.CheckErr(errors.Wrap(err, "While loading existing kubeconfig"))
		}
		log.Info("No current configuration found. Initializing...")
	}

	// Start OpenRC
	cobra.CheckErr(alpine.StartOpenRC())

	ctx := context.Background()
	if timeout > 0 {
		err = wait.PollUntilContextTimeout(ctx, time.Second*time.Duration(2), time.Duration(timeout), true, IsIkniteReady)
	} else {
		err = wait.PollUntilContextCancel(ctx, time.Second*time.Duration(2), true, IsIkniteReady)
	}

	cobra.CheckErr(err)
	log.Info("Cluster is ready")
}
