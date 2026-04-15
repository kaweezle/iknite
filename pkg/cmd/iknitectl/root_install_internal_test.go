// cSpell: words stretchr
package iknitectl

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestCreateInstallCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cmd := CreateInstallCmd(afero.NewMemMapFs())
	req.NotNil(cmd)
	req.Equal("install", cmd.Name())

	signingKeyCmd, _, err := cmd.Find([]string{"signing-key"})
	req.NoError(err)
	req.NotNil(signingKeyCmd)
	req.Equal("signing-key", signingKeyCmd.Name())
}

func TestRootOptionsAndCreateRootCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	opts := NewRootOptions()
	req.NotNil(opts)
	req.NotNil(opts.Fs)

	root := CreateRootCmd(opts)
	req.NotNil(root)
	req.Equal("iknitectl", root.Name())
	req.NotNil(root.PersistentPreRunE)

	expectedSubcommands := []string{"install", "kustomize", "application", "secrets"}
	for _, name := range expectedSubcommands {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			sub, _, err := root.Find([]string{name})
			req.NoError(err)
			req.NotNil(sub)
			req.Equal(name, sub.Name())
		})
	}
}
