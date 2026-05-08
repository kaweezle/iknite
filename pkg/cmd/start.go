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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return performStart(cmd.Context(), alpineHost, ikniteConfig, waitOptions)
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
		return false, fmt.Errorf("failed to load iknite cluster state: %w", err)
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
) error {
	err := k8s.PrepareKubernetesEnvironment(ctx, alpineHost, ikniteConfig)
	if err != nil {
		return fmt.Errorf("failed to prepare kubernetes environment: %w", err)
	}

	// If Kubernetes is already installed, check that the configuration has not
	// Changed.
	kubeClient, err := k8s.NewDefaultClient(alpineHost)
	if err == nil {
		if !k8s.IsConfigServerAddress(kubeClient, ikniteConfig.GetApiEndPoint()) {
			return fmt.Errorf("kubeconfig server address does not match iknite config API endpoint." +
				" Please reset the cluster with `iknite reset` and start again")
		}
		log.Info("Existing configuration found. Starting cluster...")
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load existing cluster admin.conf: %w", err)
		}
		log.Info("No current configuration found. Initializing...")
	}

	// Start OpenRC. This will perform `iknite init`.
	err = alpine.EnsureOpenRC(alpineHost, "default")
	if err != nil {
		return fmt.Errorf("failed to start OpenRC: %w", err)
	}
	err = waitOptions.Poll(ctx, func(ctx context.Context) (bool, error) {
		return IsIkniteReady(ctx, alpineHost)
	})
	if err != nil {
		return fmt.Errorf("cluster did not become ready in time: %w", err)
	}
	log.Info("Cluster is ready")
	return nil
}
