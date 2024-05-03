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

import (
	"fmt"
	"os"
	"time"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// configureCmd represents the start command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Gives status information on the cluster",
	Long: `Gives status information of the deployed workloads:

- Deployments
- Daemonsets
- Statefulsets
`,
	Run: performStatus,
}

var waitReadiness = false

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVarP(&waitReadiness, "wait", "w", waitReadiness, "Wait n seconds for all pods to settle")
}

var callbackCount = 0

func callback(ok bool, count int, ready []*k8s.WorkloadState, unready []*k8s.WorkloadState) {
	if callbackCount == 0 {
		fmt.Printf("\n%d workloads, %d ready, %d unready\n", count, len(ready), len(unready))
		for _, state := range ready {
			fmt.Println(state.LongString())
		}
	} else {
		if len(unready) > 0 {
			fmt.Printf("\n%d unready workloads remaining:\n", len(unready))
		} else {
			fmt.Printf("\n🎉 All workloads (%d) ready:\n", count)
			for _, state := range ready {
				fmt.Println(state.LongString())
			}
		}
	}

	for _, state := range unready {
		fmt.Println(state.LongString())
	}

	if !waitReadiness {
		os.Exit(0)
	}
	callbackCount++
}

func performStatus(cmd *cobra.Command, args []string) {

	runtime.ErrorHandlers = runtime.ErrorHandlers[:0]
	log.WithFields(log.Fields{
		"config": constants.KubernetesRootConfig,
	}).Info("Loading kube config...")

	// We need to get it from root as we will apply configuration
	config, err := k8s.LoadFromFile(constants.KubernetesRootConfig)
	cobra.CheckErr(errors.Wrap(err, "While loading local cluster configuration"))

	cobra.CheckErr(config.WaitForWorkloads(time.Second*time.Duration(0), callback))

}
