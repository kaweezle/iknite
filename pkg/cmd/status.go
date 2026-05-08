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

// cSpell: words runlevels runlevel apiserver controllermanager healthcheck logrus configurer
// cSpell: disable
import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/checkers"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

type CheckExecutorConfigurer interface {
	Configure(executor *check.CheckExecutor, ikniteConfig *v1alpha1.IkniteClusterSpec, waitOptions *utils.WaitOptions)
}

type CheckExecutorConfigFunc func(
	executor *check.CheckExecutor,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
)

func (f CheckExecutorConfigFunc) Configure(
	executor *check.CheckExecutor,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
) {
	f(executor, ikniteConfig, waitOptions)
}

func NewStatusCmd(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
	configurer CheckExecutorConfigurer,
	alpineHost host.Host,
	teaOptions ...tea.ProgramOption,
) *cobra.Command {
	if waitOptions == nil {
		waitOptions = utils.NewWaitOptions()
		waitOptions.OkResponses = 3
	}
	if configurer == nil {
		configurer = CheckExecutorConfigFunc(checkers.ConfigureIkniteClusterChecker)
	}
	if alpineHost == nil {
		alpineHost = host.NewDefaultHost()
	}
	// configureCmd represents the start command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Gives status information on the cluster",
		Long: `Gives status information of the deployed workloads:

- Deployments
- Daemonsets
- Statefulsets
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return performStatus(cmd.Context(), alpineHost, ikniteConfig, waitOptions, configurer, teaOptions...)
		},
	}

	flags := statusCmd.Flags()
	config.AddIkniteClusterFlags(flags, ikniteConfig)
	utils.AddWaitOptionsFlags(flags, waitOptions)

	return statusCmd
}

func performStatus(
	ctx context.Context,
	alpineHost host.Host,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
	configurer CheckExecutorConfigurer,
	teaOptions ...tea.ProgramOption,
) error {
	executor := check.NewCheckExecutor()
	configurer.Configure(executor, ikniteConfig, waitOptions)

	// Run all checks
	logrus.SetLevel(logrus.FatalLevel)

	checkData := checkers.CreateCheckWorkloadData(ikniteConfig, waitOptions, alpineHost)
	teaOptions = append(teaOptions, tea.WithOutput(os.Stderr))
	p := tea.NewProgram(check.NewCheckModel(ctx, executor, checkData), teaOptions...)
	tmp := os.Stdout
	defer func() { os.Stdout = tmp }()
	os.Stdout = nil
	_, err := p.Run()
	if err != nil { // nocov -- hard to cover in all test scenarios.
		return fmt.Errorf("error running checks: %w", err)
	}
	return nil
}
