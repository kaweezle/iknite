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
// cSpell: words filesys kyaml forbidigo apimachinery sirupsen
package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/kustomize"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewPrintKustomizeCmd(
	fs host.FileSystem,
	kustomizeOptions *utils.KustomizeOptions,
) *cobra.Command {
	printKustomizeCmd := &cobra.Command{
		Use:   "print",
		Short: "Print the kustomize configuration",
		Long: `Prints the kustomize configuration that would be applied to the cluster.
Checks if a /etc/iknite.d/kustomization.yaml file exists. In this case, it
prints the configuration in this directory. If there is no kustomization, it
prints the Embedded configuration that installs the following components:

- Flannel for networking.
- Kube-VIP for Load balancer services.
- Local-path provisioner to make PVCs available.
- metrics-server to make resources work on payloads.

`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := performPrintKustomize(fs, kustomizeOptions, cmd.OutOrStdout())
			if err != nil {
				return fmt.Errorf("failed to print kustomize configuration: %w", err)
			}
			return nil
		},
	}
	return printKustomizeCmd
}

func NewKustomizeCmd(
	kustomizeOptions *utils.KustomizeOptions,
	waitOptions *utils.WaitOptions,
	fs host.FileSystem,
) *cobra.Command {
	if kustomizeOptions == nil {
		kustomizeOptions = utils.NewKustomizeOptions()
	}
	if waitOptions != nil {
		waitOptions = utils.NewWaitOptions()
		// Different defaults
		waitOptions.Immediate = false
		waitOptions.Wait = false
	}
	if fs == nil {
		fs = host.NewOsFS()
	}
	kustomizeCmd := &cobra.Command{
		Use:   "kustomize",
		Short: "Kustomize the cluster",
		Long: `Apply the configuration to the cluster using kustomize.

Checks if a /etc/iknite.d/kustomization.yaml file exists. In this case, it
applies the configuration in this directory. If there is no kustomization, it
applies the Embedded configuration that installs the following components:

- Flannel for networking.
- Kube-VIP for Load balancer services.
- Local-path provisioner to make PVCs available.
- metrics-server to make resources work on payloads.

`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := performKustomize(cmd.Context(), fs, kustomizeOptions, waitOptions)
			if err != nil {
				return fmt.Errorf("failed to apply kustomize configuration: %w", err)
			}
			return nil
		},

		PreRun: func(cmd *cobra.Command, _ []string) {
			flags := cmd.Flags()

			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.ForceConfig, flags.Lookup(options.ForceConfig))
		},
	}

	utils.AddKustomizeOptionsFlags(kustomizeCmd.Flags(), kustomizeOptions)

	printCmd := NewPrintKustomizeCmd(fs, kustomizeOptions)
	inheritsFlags(kustomizeCmd.Flags(), printCmd.Flags(), options.Kustomization)
	kustomizeCmd.AddCommand(printCmd)
	return kustomizeCmd
}

func performKustomize(
	ctx context.Context,
	fs host.FileSystem,
	kustomizeOptions *utils.KustomizeOptions,
	waitOptions *utils.WaitOptions,
) error {
	// We need to get it from root as we will apply configuration
	kubeClient, err := k8s.NewClientFromFile(fs, constants.KubernetesRootConfig)
	if err != nil {
		return fmt.Errorf("while loading local cluster configuration: %w", err)
	}

	restClient, err := k8s.RESTClient(kubeClient)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	err = k8s.CheckClusterRunning(
		ctx,
		restClient,
		waitOptions.Retries,
		waitOptions.OkResponses,
		waitOptions.Interval,
	)
	if err != nil {
		return fmt.Errorf("failed to check if cluster is running: %w", err)
	}

	if err := k8s.Kustomize(ctx, kubeClient, fs, kustomizeOptions); err != nil {
		return fmt.Errorf("failed to apply kustomize configuration: %w", err)
	}

	if waitOptions.HasLoop() {
		logrus.Infof("Waiting for workloads with options: %s", waitOptions.String())
		runtime.ErrorHandlers = runtime.ErrorHandlers[:0] //nolint:reassign // disabling printing of errors to stderr
		if err := waitOptions.Poll(ctx, k8s.WorkloadsReadyConditionWithContextFunc(kubeClient, nil)); err != nil {
			return fmt.Errorf("failed to wait for workloads: %w", err)
		}
	}
	return nil
}

func performPrintKustomize(
	fs host.FileSystem,
	kustomizeOptions *utils.KustomizeOptions,
	out io.Writer,
) error {
	resources, err := provision.GetBaseKustomizationResources(
		fs,
		kustomizeOptions.Kustomization,
		kustomizeOptions.ForceEmbedded,
	)
	if err != nil {
		return fmt.Errorf("while getting kustomization resources: %w", err)
	}
	if err := kustomize.WriteToWriter(resources, out); err != nil {
		return fmt.Errorf("while writing kustomization resources: %w", err)
	}
	return nil
}
