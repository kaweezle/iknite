//nolint:testpackage // Tests validate unexported helpers used by command wiring.
package env

// cSpell: words testpackage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

type testEnv map[string]string

func (e testEnv) Getenv(key string) string {
	return e[key]
}

type testPlatform string

func (p testPlatform) GOOS() string {
	return string(p)
}

func TestDefaultConfigDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     testEnv
		os      testPlatform
		homeDir string
		want    string
	}{
		{
			name:    "linux uses xdg when present",
			env:     testEnv{"XDG_CONFIG_HOME": "/tmp/xdg"},
			os:      testPlatform("linux"),
			homeDir: "/home/alpine",
			want:    "/tmp/xdg/iknite",
		},
		{
			name:    "linux falls back to dot config",
			env:     testEnv{},
			os:      testPlatform("linux"),
			homeDir: "/home/alpine",
			want:    "/home/alpine/.config/iknite",
		},
		{
			name:    "darwin app support",
			env:     testEnv{},
			os:      testPlatform("darwin"),
			homeDir: "/Users/alpine",
			want:    "/Users/alpine/Library/Application Support/iknite",
		},
		{
			name:    "windows appdata",
			env:     testEnv{"APPDATA": `C:\\Users\\alpine\\AppData\\Roaming`},
			os:      testPlatform("windows"),
			homeDir: `C:\\Users\\alpine`,
			want:    `C:\\Users\\alpine\\AppData\\Roaming/iknite`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := defaultConfigDir(tt.env, tt.os, tt.homeDir)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestServiceInitCreatesPaths(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	svc := &Service{
		FS:       fs,
		Env:      testEnv{"XDG_CONFIG_HOME": "/tmp/xdg"},
		Platform: testPlatform("linux"),
		HomeDir: func() (string, error) {
			return "/home/alpine", nil
		},
	}

	result, err := svc.Init(&InitRequest{PrintPaths: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/tmp/xdg/iknite", result.ConfigDir)

	for _, path := range []string{
		"/tmp/xdg/iknite/auth",
		"/tmp/xdg/iknite/shared",
		"/tmp/xdg/iknite/images",
		"/tmp/xdg/iknite/clusters",
		"/tmp/xdg/iknite/auth/ca.crt",
		"/tmp/xdg/iknite/auth/ca.key",
		"/tmp/xdg/iknite/shared/values.yaml",
		"/tmp/xdg/iknite/shared/secrets.sops.yaml",
		"/tmp/xdg/iknite/shared/.sops.yaml",
	} {
		exists, existsErr := fs.Exists(path)
		require.NoError(t, existsErr)
		require.True(t, exists, path)
	}
}

func TestServiceInitRespectsConfigDir(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	svc := &Service{
		FS:       fs,
		Env:      testEnv{},
		Platform: testPlatform("linux"),
		HomeDir:  os.UserHomeDir,
	}

	result, err := svc.Init(&InitRequest{ConfigDir: "/iknite-custom"})
	require.NoError(t, err)
	require.Equal(t, "/iknite-custom", result.ConfigDir)
}
