// cSpell: words stretchr
//
//nolint:gosec // test reads temp files using controlled paths
package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/utils"
)

func TestExecuteOnExistenceVariants(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "exists.txt")
	req := require.New(t)
	req.NoError(os.WriteFile(existingFile, []byte("ok"), 0o600))

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
			path:         filepath.Join(tempDir, "missing.txt"),
			existence:    false,
			wantExecuted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			executed := false
			err := utils.ExecuteOnExistence(tt.path, tt.existence, func() error {
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

	tempDir := t.TempDir()
	present := filepath.Join(tempDir, "present.txt")
	req.NoError(os.WriteFile(present, []byte("present"), 0o600))
	absent := filepath.Join(tempDir, "absent.txt")

	executedExist := false
	err := utils.ExecuteIfExist(present, func() error {
		executedExist = true
		return nil
	})
	req.NoError(err)
	req.True(executedExist)

	executedNotExist := false
	err = utils.ExecuteIfNotExist(absent, func() error {
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
		tempDir := t.TempDir()
		src := filepath.Join(tempDir, "src.txt")
		dst := filepath.Join(tempDir, "dst.txt")
		req.NoError(os.WriteFile(src, []byte("payload"), 0o600))

		err := utils.MoveFileIfExists(src, dst)
		req.NoError(err)
		_, srcErr := os.Stat(src)
		req.Error(srcErr)
		content, readErr := os.ReadFile(dst)
		req.NoError(readErr)
		req.Equal("payload", string(content))
	})

	t.Run("missing source is no-op", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		tempDir := t.TempDir()
		src := filepath.Join(tempDir, "missing.txt")
		dst := filepath.Join(tempDir, "dst.txt")

		err := utils.MoveFileIfExists(src, dst)
		req.NoError(err)
		_, statErr := os.Stat(dst)
		req.ErrorIs(statErr, os.ErrNotExist)
	})
}

func TestEnvironmentDetectionHelpers(t *testing.T) {
	t.Parallel()

	_ = utils.IsOnWSL()
	_ = utils.IsOnIncus()
}
