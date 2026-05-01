// cSpell: words paralleltest
package iknitectl

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestCreateInstallCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cmd := CreateInstallCmd(host.NewMemMapFS())
	req.NotNil(cmd)
	req.Equal("install", cmd.Name())

	signingKeyCmd, _, err := cmd.Find([]string{"signing-key"})
	req.NoError(err)
	req.NotNil(signingKeyCmd)
	req.Equal("signing-key", signingKeyCmd.Name())
}

//nolint:paralleltest // Finding subcommands is not thread-safe, so we cannot run these tests in parallel.
func TestRootOptionsAndCreateRootCmd(t *testing.T) {
	req := require.New(t)

	opts := NewRootOptions()
	req.NotNil(opts)
	req.NotNil(opts.FileExecutor)

	root := CreateRootCmd(opts)
	req.NotNil(root)
	req.Equal("iknitectl", root.Name())
	req.NotNil(root.PersistentPreRunE)

	expectedSubcommands := []string{"install", "kustomize", "application", "secrets"}
	//nolint:paralleltest // Finding subcommands is not thread-safe, so we cannot run these tests in parallel.
	for _, name := range expectedSubcommands {
		t.Run(name, func(t *testing.T) {
			req := require.New(t)
			sub, _, err := root.Find([]string{name})
			req.NoError(err)
			req.NotNil(sub)
			req.Equal(name, sub.Name())
		})
	}
}
