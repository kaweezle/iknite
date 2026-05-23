package iknitectl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateImageCmd(t *testing.T) {
	t.Parallel()

	cmd := CreateImageCmd(NewRootDependencies())
	require.NotNil(t, cmd)

	for _, sub := range []string{"inspect", "pull"} {
		found, _, err := cmd.Find([]string{sub})
		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, sub, found.Name())
	}
}
