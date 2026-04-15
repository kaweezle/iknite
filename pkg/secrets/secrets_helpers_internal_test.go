// cSpell: words stretchr
package secrets

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestBuildSecretsPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		wantLen int
	}{
		{name: "valid path", path: "github.api_token", wantErr: false, wantLen: 3},
		{name: "single token", path: "token", wantErr: false, wantLen: 2},
		{name: "empty token", path: "github..token", wantErr: true, wantLen: 0},
		{name: "blank path", path: "", wantErr: true, wantLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			parts, err := buildSecretsPath(tt.path)
			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.Equal("data", parts[0])
			req.Len(parts, tt.wantLen)
		})
	}
}

func TestPathExpansionAndDisplay(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	home := "/home/tester"
	req.Equal(home, expandHomePath(home, "~"))
	req.Equal(filepath.Join(home, ".ssh", "id_ed25519"), expandHomePath(home, "~/.ssh/id_ed25519"))
	req.Equal("/tmp/k", expandHomePath(home, "/tmp/k"))

	req.Equal("~", displayPath(home, home))
	req.Equal(filepath.Join("~", ".ssh", "id_ed25519"), displayPath(home, filepath.Join(home, ".ssh", "id_ed25519")))
	req.Equal("/etc/hosts", displayPath(home, "/etc/hosts"))
}

func TestResolveSecretsInitPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		opts    *Options
		name    string
		wantErr bool
	}{
		{
			name: "defaults use home dir key",
			opts: &Options{
				Fs:          afero.NewMemMapFs(),
				HomeDir:     "/home/tester",
				SecretsFile: filepath.Join("workspace", DefaultSecretsFile),
			},
			wantErr: false,
		},
		{
			name: "missing home dir with no key file errors",
			opts: &Options{
				Fs:          afero.NewMemMapFs(),
				SecretsFile: DefaultSecretsFile,
			},
			wantErr: true,
		},
		{
			name: "custom key file supported",
			opts: &Options{
				Fs:          afero.NewMemMapFs(),
				HomeDir:     "/home/tester",
				SecretsFile: DefaultSecretsFile,
				KeyFile:     "~/.ssh/custom",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			paths, err := resolveSecretsInitPaths(tt.opts)
			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.NotNil(paths)
			req.NotEmpty(paths.keyFile)
			req.NotEmpty(paths.publicKeyFile)
			req.NotEmpty(paths.sopsConfigFile)
		})
	}
}

func TestEncryptAndLoadSecretsErrorPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	_, err := encryptSecretsPlaintext(DefaultSecretsFile, []byte(""), "invalid-recipient")
	req.Error(err)

	opts := &Options{Fs: afero.NewMemMapFs(), SecretsFile: filepath.Join("tmp", "missing.yaml")}
	_, err = loadAndDecryptSecrets(opts)
	req.Error(err)
	req.Contains(err.Error(), "secrets file not found")
}
