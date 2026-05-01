package checkers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/checkers"
	"github.com/kaweezle/iknite/pkg/utils"
)

func TestConfigureIkniteClusterChecker(t *testing.T) {
	t.Parallel()

	req := require.New(t)

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}
	waitOptions := utils.NewWaitOptions()
	executor := check.NewCheckExecutor()

	checkers.ConfigureIkniteClusterChecker(executor, ikniteConfig, waitOptions)
	req.Len(executor.Checks, 4)
}

func TestNewConfigurationCheckPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}
	configCheck := checkers.NewConfigurationCheckPhase(ikniteConfig)
	req.NotNil(configCheck)
	// get last sub-check
	req.NotEmpty(configCheck.SubChecks)
	last := configCheck.SubChecks[len(configCheck.SubChecks)-1]
	req.Contains(last.Name, "kine")

	ikniteConfig.UseEtcd = true
	configCheck = checkers.NewConfigurationCheckPhase(ikniteConfig)
	req.NotNil(configCheck)
	// get last sub-check
	req.NotEmpty(configCheck.SubChecks)
	last = configCheck.SubChecks[len(configCheck.SubChecks)-1]
	req.Contains(last.Name, "etcd")
}
