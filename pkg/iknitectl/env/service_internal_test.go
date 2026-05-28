// cSpell: words testpackage testconfig
package env

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/testutil"
)

type testEnv map[string]string

func (e testEnv) Getenv(key string) string {
	return e[key]
}

func TestDefaultConfigDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      testEnv
		os       string
		username string
		want     string
	}{
		{
			name:     "linux uses xdg when present",
			env:      testEnv{"XDG_CONFIG_HOME": "/tmp/xdg"},
			os:       "linux",
			username: "alpine",
			want:     "/tmp/xdg/iknite",
		},
		{
			name:     "linux falls back to dot config",
			env:      testEnv{},
			os:       "linux",
			username: "alpine",
			want:     "/home/alpine/.config/iknite",
		},
		{
			name:     "darwin app support",
			env:      testEnv{},
			os:       "darwin",
			username: "alpine",
			want:     "/Users/alpine/Library/Application Support/iknite",
		},
		{
			name:     "windows appdata",
			env:      testEnv{},
			os:       "windows",
			username: "alpine",
			want:     `C:\Users\alpine\AppData\Roaming\iknite`,
		},
		{
			name:     "windows appdata",
			env:      testEnv{"APPDATA": "C:\\test"},
			os:       "windows",
			username: "alpine",
			want:     `C:\test\iknite`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := testutil.NewDummyPlatformHost(tt.os, tt.username, tt.env)
			got, err := config.DefaultConfigDir(fs)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestServiceInitCreatesPaths(t *testing.T) {
	t.Parallel()

	fs := testutil.NewDummyPlatformHost("linux", "alpine", testEnv{"XDG_CONFIG_HOME": "/tmp/xdg"})
	svc := &Service{
		FS:     fs,
		Logger: testutil.TestLogger(t),
	}

	result, err := svc.Init(&InitRequest{PrintPaths: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Paths)
	require.Equal(t, "/tmp/xdg/iknite", result.Paths.Root)

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

func TestServiceInit(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := testutil.NewDummyUserHost()
	svc := &Service{
		FS:     fs,
		Logger: testutil.TestLogger(t),
	}
	configDir, err := fs.UserConfigDir()
	req.NoError(err)

	result, err := svc.Init(&InitRequest{})
	require.NoError(t, err)
	require.NotNil(t, result.Paths)
	expectedRoot := fs.JoinPath(configDir, constants.IkniteConfName)
	require.Equal(t, expectedRoot, result.Paths.Root)
}

func TestServiceInitRespectsConfigDir(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := testutil.NewDummyUserHost()
	svc := &Service{
		FS:     fs,
		Logger: testutil.TestLogger(t),
		Config: &config.Config{},
	}
	opts := config.NewConfigOptions(fs)
	opts.ConfigDir = "/tmp/testconfig"
	req.NoError(opts.Resolve(fs, svc.Config))

	result, err := svc.Init(&InitRequest{})
	req.NoError(err)
	req.NotNil(result.Paths)
	req.Equal("/tmp/testconfig/auth", result.Paths.Auth)
}
