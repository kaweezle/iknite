// cSpell: words paralleltest
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultIkniteConf(t *testing.T) {
	req := require.New(t)

	home := t.TempDir()
	t.Setenv("HOME", home)

	path := defaultIkniteConf()
	req.Equal(filepath.Join(home, ".kube", "iknite.conf"), path)
}

func TestNewInfoStatusCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cmd := NewInfoStatusCmd()
	req.NotNil(cmd)
	req.Equal("status", cmd.Name())
	flag := cmd.Flags().Lookup(ikniteConfigFlag)
	req.NotNil(flag)
	req.NotEmpty(flag.Value.String())
}

//nolint:paralleltest // mutates environment
func TestDefaultIkniteConf_NoHome(t *testing.T) {
	req := require.New(t)

	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		req.NoError(os.Setenv("HOME", oldHome))
	})
	req.NoError(os.Unsetenv("HOME"))

	_ = defaultIkniteConf()
}
