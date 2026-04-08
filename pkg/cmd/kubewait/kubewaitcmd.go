/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

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

// Package kubewait implements the kubewait command.
// It waits for Kubernetes resources in specified namespaces to become ready
// using kstatus (one goroutine per namespace), then optionally clones and runs
// a bootstrap repository script.
package kubewait

// cSpell: words godotenv clientcmd apimachinery kstatus errorf sirupsen joho metav1

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cmdUtil "github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/kubewait"
)

// CreateKubewaitCmd creates the root cobra command for kubewait.
func CreateKubewaitCmd(out io.Writer) *cobra.Command {
	opts := kubewait.NewOptions()

	cmd := &cobra.Command{
		Use:   "kubewait [namespaces...]",
		Short: "Wait for Kubernetes resources to be ready",
		Long: `kubewait waits for all deployments, statefulsets and daemonsets in the
specified namespaces to reach a ready state according to kstatus.

Each namespace is watched concurrently. If a namespace is not yet present at
invocation time the goroutine waits for its creation, then applies a short
grace period to let resources appear before polling their readiness.

If no namespaces are given, all namespaces present in the cluster at invocation
time are watched.

After all resources are ready, an optional bootstrap repository is cloned
(when --bootstrap-repo-url and --bootstrap-repo-ref are provided) and then the
bootstrap script inside that directory is executed.

Examples:
  # Wait for resources in all namespaces
  kubewait

  # Wait for specific namespaces
  kubewait kube-system default

  # Clone and run a bootstrap script after resources are ready
  kubewait --bootstrap-repo-url git@github.com:org/repo.git --bootstrap-repo-ref main

  # Use a specific kubeconfig
  kubewait --kubeconfig ~/.kube/config kube-system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setUpLogs(out, opts.Verbosity, opts.JSONLogs); err != nil {
				return err
			}

			return kubewait.RunKubewait(cmd.Context(), opts, args)
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			rootCmd := cmd.Root()
			err := cmdUtil.InitializeConfiguration(rootCmd)
			cobra.CheckErr(err)
			cmdUtil.ApplyViperConfigToFlags(rootCmd, viper.GetViper())
			ok, err := opts.ReadEnvFile()
			cobra.CheckErr(err)
			if ok {
				// Re-apply config to flags to override with env file values if needed
				cmdUtil.ApplyViperConfigToFlags(rootCmd, viper.GetViper())
			}
			return nil
		},
	}

	flags := cmd.Flags()
	kubewait.AddKubewaitFlags(flags, opts)
	cmdUtil.BindFlagsToViper(cmd, viper.GetViper())

	return cmd
}

// Execute is the entry point called from main.
func Execute() {
	cobra.CheckErr(CreateKubewaitCmd(os.Stdout).Execute())
}

// setUpLogs configures logrus output and level.
func setUpLogs(out io.Writer, level log.Level, jsonFormat bool) error {
	log.SetOutput(out)
	if jsonFormat {
		log.SetFormatter(&log.JSONFormatter{})
	}
	log.SetLevel(level)
	return nil
}
