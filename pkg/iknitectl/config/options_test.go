package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestResolveSetsAndInitializesDatabasePath(t *testing.T) {
	t.Parallel()

	fs := testutil.NewDummyPlatformHost("linux", "alpine", map[string]string{"XDG_CONFIG_HOME": "/tmp/xdg"})
	cfg := &config.Config{}
	err := config.NewConfigOptions(fs).Resolve(fs, cfg)
	require.NoError(t, err)

	require.Equal(t, "/tmp/xdg/iknite", cfg.Root)
	require.Equal(t, "/tmp/xdg/iknite/iknite.db", cfg.Database)
}
