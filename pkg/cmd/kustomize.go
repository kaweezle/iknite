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
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/kustomize"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewPrintKustomizeCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
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
		Run: func(_ *cobra.Command, _ []string) {
			performPrintKustomize(ikniteConfig)
		},
		PreRun: func(cmd *cobra.Command, _ []string) {
			flags := cmd.Flags()
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.Kustomization, flags.Lookup(options.Kustomization))
		},
	}
	return printKustomizeCmd
}

func NewKustomizeCmd(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	kustomizeOptions *utils.KustomizeOptions,
	waitOptions *utils.WaitOptions,
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
		Run: func(_ *cobra.Command, _ []string) {
			performKustomize(ikniteConfig, kustomizeOptions, waitOptions)
		},

		PreRun: func(cmd *cobra.Command, _ []string) {
			flags := cmd.Flags()
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.Kustomization, flags.Lookup(options.Kustomization))

			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.ForceConfig, flags.Lookup(options.ForceConfig))
		},
	}

	config.AddIkniteClusterFlags(kustomizeCmd.Flags(), ikniteConfig)
	utils.AddKustomizeOptionsFlags(kustomizeCmd.Flags(), kustomizeOptions)

	printCmd := NewPrintKustomizeCmd(ikniteConfig)
	inheritsFlags(kustomizeCmd.Flags(), printCmd.Flags(), options.Kustomization)
	kustomizeCmd.AddCommand(printCmd)
	return kustomizeCmd
}

func performKustomize(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	kustomizeOptions *utils.KustomizeOptions,
	waitOptions *utils.WaitOptions,
) {
	// We need to get it from root as we will apply configuration
	kubeConfig, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("while loading local cluster configuration: %w", err))
	}
	// Make wait finish when the process receives an interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	err = kubeConfig.CheckClusterRunning(
		ctx,
		waitOptions.Retries,
		waitOptions.OkResponses,
		waitOptions.Interval,
	)
	cobra.CheckErr(err)

	cobra.CheckErr(kubeConfig.Kustomize(ctx, ikniteConfig.Kustomization, kustomizeOptions.ForceConfig))

	if waitOptions.HasLoop() {
		logrus.Infof("Waiting for workloads with options: %s", waitOptions.String())
		runtime.ErrorHandlers = runtime.ErrorHandlers[:0] //nolint:reassign // disabling printing of errors to stderr
		cobra.CheckErr(waitOptions.Poll(ctx, kubeConfig.RESTClient().WorkloadsReadyConditionWithContextFunc(nil)))
	}
}

func performPrintKustomize(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	resources, err := provision.GetBaseKustomizationResources(ikniteConfig.Kustomization)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("while getting kustomization resources: %w", err))
	}
	cobra.CheckErr(kustomize.WriteToWriter(resources, os.Stdout))
}
