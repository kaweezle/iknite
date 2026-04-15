package host_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestExecuteOnExistenceVariants(t *testing.T) {
	t.Parallel()

	existingFile := "exists.txt"

	tests := []struct {
		name         string
		path         string
		existence    bool
		wantExecuted bool
	}{
		{name: "execute when file exists", path: existingFile, existence: true, wantExecuted: true},
		{name: "skip when file exists but expected missing", path: existingFile, existence: false, wantExecuted: false},
		{
			name:         "execute when file is missing",
			path:         "missing.txt",
			existence:    false,
			wantExecuted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := host.NewMemMapFS()
			req.NoError(fs.WriteFile(existingFile, []byte("ok"), 0o600))

			executed := false
			err := host.ExecuteOnExistence(fs, tt.path, tt.existence, func() error {
				executed = true
				return nil
			})

			req.NoError(err)
			req.Equal(tt.wantExecuted, executed)
		})
	}
}

func TestExecuteIfExistAndNotExist(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	present := "present.txt"
	req.NoError(fs.WriteFile(present, []byte("present"), 0o600))
	absent := "absent.txt"

	executedExist := false
	err := host.ExecuteIfExist(fs, present, func() error {
		executedExist = true
		return nil
	})
	req.NoError(err)
	req.True(executedExist)

	executedNotExist := false
	err = host.ExecuteIfNotExist(fs, absent, func() error {
		executedNotExist = true
		return nil
	})
	req.NoError(err)
	req.True(executedNotExist)
}

func TestMoveFileIfExists(t *testing.T) {
	t.Parallel()

	t.Run("move existing file", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := host.NewMemMapFS()
		src := "src.txt"
		dst := "dst.txt"
		req.NoError(fs.WriteFile(src, []byte("payload"), 0o600))

		err := host.MoveFileIfExists(fs, src, dst)
		req.NoError(err)
		_, srcErr := fs.Stat(src)
		req.Error(srcErr)
		content, readErr := fs.ReadFile(dst)
		req.NoError(readErr)
		req.Equal("payload", string(content))
	})

	t.Run("missing source is no-op", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := host.NewMemMapFS()
		src := "missing.txt"
		dst := "dst.txt"

		err := host.MoveFileIfExists(fs, src, dst)
		req.NoError(err)
		_, statErr := fs.Stat(dst)
		req.ErrorIs(statErr, os.ErrNotExist)
	})
}

func TestEnvironmentDetectionHelpers(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := host.NewMemMapFS()

	ok := host.IsOnWSL(fs)
	req.False(ok)

	req.NoError(fs.MkdirAll("/run/WSL", 0o755))
	ok = host.IsOnWSL(fs)
	req.True(ok)

	ok = host.IsOnIncus(fs)
	req.False(ok)
	req.NoError(fs.MkdirAll("/dev/.lxc/proc", 0o755))
	ok = host.IsOnIncus(fs)
	req.True(ok)
}
