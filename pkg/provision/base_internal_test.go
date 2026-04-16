// cSpell: words paralleltest kyaml filesys
package provision

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestIsBaseKustomizationAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepare func(req *require.Assertions, fs host.FileSystem) string
		name    string
		want    bool
	}{
		{
			name: "url path",
			prepare: func(_ *require.Assertions, _ host.FileSystem) string {
				return "https://example.org/repo"
			},
			want: true,
		},
		{
			name: "local directory with kustomization",
			prepare: func(req *require.Assertions, fs host.FileSystem) string {
				dir := "base"
				req.NoError(fs.MkdirAll(dir, 0o755))
				req.NoError(
					fs.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("resources: []\n"), 0o600),
				)
				return dir
			},
			want: true,
		},
		{
			name: "local directory missing file",
			prepare: func(_ *require.Assertions, _ host.FileSystem) string {
				return "missing"
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := host.NewMemMapFS()

			dir := tt.prepare(req, fs)
			ok, err := isBaseKustomizationAvailable(fs, dir)
			req.NoError(err)
			req.Equal(tt.want, ok)
		})
	}
}

func TestCreateTempKustomizeDirectory(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := filesys.MakeFsInMemory()
	err := fs.Mkdir("base")
	req.NoError(err)

	err = createTempKustomizeDirectory(&content, fs, "base", "base")
	req.NoError(err)
	req.True(fs.Exists("base/kustomization.yaml"))
}

//nolint:paralleltest // kustomize uses global state disallowing parallel tests
func TestGetBaseKustomizationResources(t *testing.T) {
	tests := []struct {
		prepare       func(req *require.Assertions, fs host.FileSystem) string
		name          string
		forceEmbedded bool
	}{
		{
			name: "forced embedded",
			prepare: func(_ *require.Assertions, _ host.FileSystem) string {
				return "/does/not/exist"
			},
			forceEmbedded: true,
		},
		{
			name: "fallback embedded when missing",
			prepare: func(_ *require.Assertions, _ host.FileSystem) string {
				return "/does/not/exist"
			},
			forceEmbedded: false,
		},
		{
			name: "custom local kustomization",
			prepare: func(req *require.Assertions, fs host.FileSystem) string {
				dir := "/base"
				req.NoError(fs.MkdirAll(dir, 0o755))
				req.NoError(
					fs.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("resources:\n- cm.yaml\n"), 0o600),
				)
				req.NoError(
					fs.WriteFile(
						filepath.Join(dir, "cm.yaml"),
						[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"),
						0o600,
					),
				)
				return dir
			},
			forceEmbedded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			fs := host.NewMemMapFS()

			resources, err := GetBaseKustomizationResources(fs, tt.prepare(req, fs), tt.forceEmbedded)
			req.NoError(err)
			req.NotNil(resources)
			req.Positive(resources.Size())
		})
	}
}
