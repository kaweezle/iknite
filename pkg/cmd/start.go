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
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func NewStartCmd(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
	alpineHost host.Host,
) *cobra.Command {
	if waitOptions == nil {
		waitOptions = utils.NewWaitOptions()
	}
	if alpineHost == nil {
		alpineHost = host.NewDefaultHost()
	}
	// startCmd represents the start command
	startCmd := &cobra.Command{
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
		Run: func(cmd *cobra.Command, _ []string) {
			performStart(cmd.Context(), alpineHost, ikniteConfig, waitOptions)
		},
	}
	flags := startCmd.Flags()

	config.AddIkniteClusterFlags(flags, ikniteConfig)
	utils.AddWaitOptionsFlags(flags, waitOptions)

	return startCmd
}

func IsIkniteReady(_ context.Context, fs host.FileSystem) (bool, error) {
	cluster, err := v1alpha1.LoadIkniteCluster(fs)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("failed to load iknite cluster: %w", err)
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

func performStart(
	ctx context.Context,
	alpineHost host.Host,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
) {
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ctx, alpineHost, ikniteConfig))

	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	kubeClient, err := k8s.NewDefaultClient(alpineHost)
	if err == nil {
		if k8s.IsConfigServerAddress(kubeClient, ikniteConfig.GetApiEndPoint()) {
			log.Info("Kubeconfig already exists")
		} else {
			// If the configuration has changed, we stop and disable the kubelet
			// that may be started and clean the configuration, i.e. delete
			// certificates and manifests.
			log.Info("Kubernetes configuration has changed. Resetting...")
			cmd := newCmdReset(os.Stdin, os.Stdout, nil, nil)
			cobra.CheckErr(cmd.RunE(cmd, []string{}))
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			cobra.CheckErr(fmt.Errorf("while loading existing kubeconfig: %w", err))
		}
		log.Info("No current configuration found. Initializing...")
	}

	// Start OpenRC. This will perform `iknite init`.
	cobra.CheckErr(alpine.EnsureOpenRC(alpineHost, "default"))
	cobra.CheckErr(waitOptions.Poll(ctx, func(ctx context.Context) (bool, error) {
		return IsIkniteReady(ctx, alpineHost)
	}))
	log.Info("Cluster is ready")
}
