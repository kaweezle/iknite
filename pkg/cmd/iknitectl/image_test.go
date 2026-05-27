package iknitectl_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	iknitectl "github.com/kaweezle/iknite/pkg/cmd/iknitectl"
	"github.com/kaweezle/iknite/pkg/iknitectl/base"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestCreateImageCmd(t *testing.T) {
	t.Parallel()

	h := testutil.NewDummyUserHost()
	logger := testutil.TestLogger(t)
	c := config.NewConfigOptions(h)
	baseService := base.NewService(h, logger, c)
	cmd := iknitectl.CreateImageCmd(baseService)
	require.NotNil(t, cmd)

	for _, sub := range []string{"inspect", "pull"} {
		found, _, err := cmd.Find([]string{sub})
		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, sub, found.Name())
	}
}
