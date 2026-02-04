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
// cSpell: words filesys kyaml forbidigo
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
)

var (
	kustomization                = constants.DefaultKustomization
	waitTimeout                  = 0
	clusterCheckWaitMilliseconds = 500
	clusterCheckRetries          = 1
	clusterCheckOkResponses      = 1
)

func NewPrintKustomizeCmd() *cobra.Command {
	printKustomizeCmd := &cobra.Command{
		Use:   "print",
		Short: "Print the kustomize configuration",
		Long: `Prints the kustomize configuration that would be applied to the cluster.
Checks if a /etc/iknite.d/kustomization.yaml file exists. In this case, it
prints the configuration in this directory. If there is no kustomization, it
prints the Embedded configuration that installs the following components:

- Flannel for networking.
- MetalLB for Load balancer services.
- Local-path provisioner to make PVCs available.
- metrics-server to make resources work on payloads.

`,
		Run: performPrintKustomize,
		PreRun: func(cmd *cobra.Command, _ []string) {
			flags := cmd.Flags()
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.Kustomization, flags.Lookup(options.Kustomization))
		},
	}
	return printKustomizeCmd
}

func NewKustomizeCmd() *cobra.Command {
	kustomizeCmd := &cobra.Command{
		Use:   "kustomize",
		Short: "Kustomize the cluster",
		Long: `Apply the configuration to the cluster using kustomize.

Checks if a /etc/iknite.d/kustomization.yaml file exists. In this case, it
applies the configuration in this directory. If there is no kustomization, it
applies the Embedded configuration that installs the following components:

- Flannel for networking.
- MetalLB for Load balancer services.
- Local-path provisioner to make PVCs available.
- metrics-server to make resources work on payloads.

`,
		Run: performKustomize,
		PreRun: func(cmd *cobra.Command, _ []string) {
			flags := cmd.Flags()
			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.Kustomization, flags.Lookup(options.Kustomization))

			_ = viper.BindPFlag( //nolint:errcheck // flag exists
				config.ForceConfig, flags.Lookup(options.ForceConfig))
		},
	}

	initializeKustomization(kustomizeCmd.Flags())
	kustomizeCmd.PersistentFlags().
		StringVarP(&kustomization, options.Kustomization, "d", constants.DefaultKustomization,
			"The directory to look for kustomization. Can be an URL")

	kustomizeCmd.AddCommand(NewPrintKustomizeCmd())
	return kustomizeCmd
}

func initializeKustomization(flagSet *flag.FlagSet) {
	flagSet.IntVarP(
		&waitTimeout,
		options.Wait,
		"w",
		waitTimeout,
		"Wait n seconds for all pods to settle",
	)
	flagSet.BoolP(
		options.ForceConfig,
		"C",
		false,
		"Force configuration even if it has already occurred",
	)
	flagSet.IntVar(
		&clusterCheckWaitMilliseconds,
		options.ClusterCheckWait,
		clusterCheckWaitMilliseconds,
		"Milliseconds to wait between each cluster check",
	)
	flagSet.IntVar(&clusterCheckRetries, options.ClusterCheckRetries, clusterCheckRetries,
		"Number of tries to access the cluster")
	flagSet.IntVar(
		&clusterCheckOkResponses,
		options.ClusterCheckOkResponses,
		clusterCheckOkResponses,
		"Number of Ok response to receive before proceeding",
	)
}

func performKustomize(_ *cobra.Command, _ []string) {
	ip, err := utils.GetOutboundIP()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("while getting IP address: %w", err))
	}

	// We need to get it from root as we will apply configuration
	kubeConfig, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("while loading local cluster configuration: %w", err))
	}
	err = kubeConfig.CheckClusterRunning(
		clusterCheckRetries,
		clusterCheckOkResponses,
		clusterCheckWaitMilliseconds,
	)
	cobra.CheckErr(err)

	force := viper.GetBool(config.ForceConfig)
	kustomization := viper.GetString(config.Kustomization)
	cobra.CheckErr(kubeConfig.DoKustomization(ip, kustomization, force, waitTimeout))
}

func performPrintKustomize(_ *cobra.Command, _ []string) {
	kustomization := viper.GetString(config.Kustomization)
	if kustomization == "" {
		kustomization = constants.DefaultKustomization
	}
	if ok, err := provision.IsBaseKustomizationAvailable(kustomization); ok {
		var resources resmap.ResMap
		resources, err = provision.RunKustomizations(filesys.MakeFsOnDisk(), kustomization)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("while applying local kustomization: %w", err))
		}
		// Dump resources as YAML to stdout
		var out []byte
		out, err = resources.AsYaml()
		cobra.CheckErr(err)
		fmt.Println(string(out)) //nolint:forbidigo // printing is expected here
	} else {
		cobra.CheckErr(fmt.Errorf("bad kustomization: %s: %w", kustomization, err))
	}
}
