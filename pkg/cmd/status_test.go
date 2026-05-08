// cSpell: words paralleltest configurer testutil charmbracelet bubbletea sirupsen
//
//nolint:paralleltest // mutates viper
package cmd_test

import (
	"bytes"
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/cmd"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
	"github.com/kaweezle/iknite/pkg/utils"
)

func simpleConfigurer(
	executor *check.CheckExecutor,
	_ *v1alpha1.IkniteClusterSpec,
	_ *utils.WaitOptions,
) {
	executor.AddCheck(&check.Check{
		Name: "simple",
		CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
			logrus.Info("Running simple check")
			return true, "simple check passed", nil
		},
	})
}

func TestStatusCommand(t *testing.T) {
	req := require.New(t)

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}
	waitOptions := utils.NewWaitOptions()
	fs := host.NewMemMapFS()
	mockHost, err := testutil.NewDummyHost(fs, &testutil.DummyHostOptions{})
	req.NoError(err)
	input := &bytes.Buffer{}
	command := cmd.NewStatusCmd(
		ikniteConfig,
		waitOptions,
		cmd.CheckExecutorConfigFunc(simpleConfigurer),
		mockHost,
		tea.WithInput(input),
		tea.WithoutRenderer(),
	)
	err = command.ExecuteContext(t.Context())
	req.NoError(err)
}
