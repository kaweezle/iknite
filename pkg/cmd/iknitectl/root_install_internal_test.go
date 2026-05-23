// cSpell: words paralleltest
//
//nolint:errcheck // not checking setEnv
package iknitectl

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestCreateWorkspaceCmd(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fileExecutor, ok := host.NewMemMapFS().(host.UserHost)
	req.True(ok, "MemMapFS should implement UserHost")

	cmd := CreateWorkspaceCmd(fileExecutor, &bytes.Buffer{})
	req.NotNil(cmd)
	req.Equal("workspace", cmd.Name())

	for _, name := range []string{"application", "kustomize", "secrets"} {
		sub, _, err := cmd.Find([]string{name})
		req.NoError(err)
		req.NotNil(sub)
		req.Equal(name, sub.Name())
	}
}

//nolint:paralleltest // Finding subcommands is not thread-safe, so we cannot run these tests in parallel.
func TestRootOptionsAndCreateRootCmd(t *testing.T) {
	req := require.New(t)

	opts := NewRootOptions()
	req.NotNil(opts)
	req.NotNil(opts.Dependencies)

	root := CreateRootCmd(opts)
	req.NotNil(root)
	req.Equal("iknitectl", root.Name())
	req.NotNil(root.PersistentPreRunE)

	expectedSubcommands := []string{"env", "image", "cluster", "workspace", "auth", "backend"}
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

//nolint:paralleltest // Messing with home
func TestCreateRootCmd(t *testing.T) {
	req := require.New(t)

	cmd := CreateRootCmd(nil)
	req.NotNil(cmd)
	req.Equal("iknitectl", cmd.Name())
}

//nolint:paralleltest // Messing with home
func TestRunRootCmd_Path(t *testing.T) {
	req := require.New(t)

	fileExecutor, ok := host.NewMemMapFS().(host.FileExecutor)
	req.True(ok, "MemMapFS should implement FileExecutor")
	h, hostOK := fileExecutor.(host.Host)
	req.True(hostOK, "MemMapFS should implement Host")

	out := &bytes.Buffer{}
	options := &RootOptions{
		Dependencies: &RootDependencies{Host: h},
		out:          out,
	}
	cmd := CreateRootCmd(options)
	req.NotNil(cmd)

	cmd.SetArgs([]string{"workspace", "kustomize", "nonexistent"})

	err := cmd.ExecuteContext(t.Context())
	req.Error(err)
	req.Contains(err.Error(), "kustomization directory does not exist")
	req.Contains(out.String(), "Usage:\n  iknitectl workspace kustomize <directory> [destination]")
}

//nolint:paralleltest // Messing with home
func TestRunRootCmd_ConfigError(t *testing.T) {
	req := require.New(t)

	fileExecutor, ok := host.NewMemMapFS().(host.FileExecutor)
	req.True(ok, "MemMapFS should implement FileExecutor")
	h, hostOK := fileExecutor.(host.Host)
	req.True(hostOK, "MemMapFS should implement Host")

	out := &bytes.Buffer{}
	options := &RootOptions{
		Dependencies: &RootDependencies{Host: h},
		out:          out,
	}
	cmd := CreateRootCmd(options)
	req.NotNil(cmd)

	cmd.SetArgs([]string{"workspace", "kustomize", "nonexistent"})

	oldHome := os.Getenv("HOME")
	oldXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDGConfigHome)
	})
	os.Unsetenv("HOME")
	os.Unsetenv("XDG_CONFIG_HOME")

	err := cmd.ExecuteContext(t.Context())
	req.Error(err)
	req.Contains(err.Error(), "failed to initialize configuration")
}
