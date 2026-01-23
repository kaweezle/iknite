package utils_test

// cSpell: words testdir

// cSpell: disable
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitfield/script"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/utils"
)

const basicTestPath = "test.txt"

var basicTests = []struct {
	setupFS func(t *testing.T) utils.FileSystem
	name    string
}{
	{
		name:    "TempFS",
		setupFS: setupTempFS,
	},
	{
		name:    "MemFS",
		setupFS: setupMemFS,
	},
}

// cSpell: enable

func setupTempFSOnDirectory(t *testing.T, tempDir string) utils.FileSystem {
	t.Helper()
	baseFs := afero.NewOsFs()
	fs := utils.NewBasePathFS(baseFs, tempDir)
	return fs
}

// setupMemFS creates an in-memory filesystem.
func setupMemFS(t *testing.T) utils.FileSystem {
	t.Helper()
	return utils.NewMemMapFS()
}

// setupTempFS creates a filesystem backed by a temporary directory.
func setupTempFS(t *testing.T) utils.FileSystem {
	t.Helper()
	return setupTempFSOnDirectory(t, t.TempDir())
}

func TestFileSystem_WriteFile_ReadFile(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := []byte("test content\n")

			// Test WriteFile
			err := fs.WriteFile(basicTestPath, testData, 0o644)
			require.NoError(t, err)

			// Test ReadFile
			content, err := fs.ReadFile(basicTestPath)
			require.NoError(t, err)
			assert.Equal(t, testData, content)
		})
	}
}

func TestFileSystem_WriteFile_NonExistentDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupFS     func(t *testing.T) utils.FileSystem
		name        string
		expectError bool
	}{
		{
			name:        "TempFS",
			setupFS:     setupTempFS,
			expectError: true,
		},
		{
			// In MemMapFs, writing to a non-existent directory creates the directory automatically
			name:        "MemFS",
			setupFS:     setupMemFS,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testPath := "nonexistent/dir/test.txt"
			err := fs.WriteFile(testPath, []byte("test"), 0o644)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileSystem_ReadFile_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			_, err := fs.ReadFile("nonexistent.txt")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_Stat(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := []byte("test content")

			err := fs.WriteFile(basicTestPath, testData, 0o644)
			require.NoError(t, err)

			info, err := fs.Stat(basicTestPath)
			require.NoError(t, err)
			assert.Equal(t, basicTestPath, info.Name())
			assert.Equal(t, int64(len(testData)), info.Size())
			assert.False(t, info.IsDir())
		})
	}
}

func TestFileSystem_Stat_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			_, err := fs.Stat("nonexistent.txt")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_Create(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			file, err := fs.Create(basicTestPath)
			require.NoError(t, err)
			assert.NotNil(t, file)
			defer file.Close() //nolint:errcheck // ignore close error in test

			_, err = file.WriteString("test")
			require.NoError(t, err)

			// Verify file exists
			exists, err := fs.Exists(basicTestPath)
			require.NoError(t, err)
			assert.True(t, exists)
		})
	}
}

func TestFileSystem_Open(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := []byte("test content")

			err := fs.WriteFile(basicTestPath, testData, 0o644)
			require.NoError(t, err)

			file, err := fs.Open(basicTestPath)
			require.NoError(t, err)
			assert.NotNil(t, file)
			defer file.Close() //nolint:errcheck // ignore close error in test

			buf := make([]byte, len(testData))
			n, err := file.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf)
		})
	}
}

func TestFileSystem_Open_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			_, err := fs.Open("nonexistent.txt")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_OpenFile(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := []byte("initial content")

			err := fs.WriteFile(basicTestPath, testData, 0o644)
			require.NoError(t, err)

			// Open for append
			file, err := fs.OpenFile(basicTestPath, os.O_APPEND|os.O_WRONLY, 0o644)
			require.NoError(t, err)
			defer file.Close() //nolint:errcheck // ignore close error in test

			appendData := []byte(" appended")
			_, err = file.Write(appendData)
			require.NoError(t, err)

			// Verify content
			content, err := fs.ReadFile(basicTestPath)
			require.NoError(t, err)
			assert.Equal(t, append(testData, appendData...), content)
		})
	}
}

func TestFileSystem_Remove(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			err := fs.WriteFile(basicTestPath, []byte("test"), 0o644)
			require.NoError(t, err)

			err = fs.Remove(basicTestPath)
			require.NoError(t, err)

			exists, err := fs.Exists(basicTestPath)
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestFileSystem_Remove_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			err := fs.Remove("nonexistent.txt")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_RemoveAll(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			// Create directory with files
			err := fs.MkdirAll("testdir/subdir", 0o755)
			require.NoError(t, err)

			err = fs.WriteFile("testdir/file1.txt", []byte("test1"), 0o644)
			require.NoError(t, err)

			err = fs.WriteFile("testdir/subdir/file2.txt", []byte("test2"), 0o644)
			require.NoError(t, err)

			// Remove all
			err = fs.RemoveAll("testdir")
			require.NoError(t, err)

			exists, err := fs.Exists("testdir")
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestFileSystem_MkdirAll(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			dirPath := "dir1/dir2/dir3"
			err := fs.MkdirAll(dirPath, 0o755)
			require.NoError(t, err)

			exists, err := fs.DirExists(dirPath)
			require.NoError(t, err)
			assert.True(t, exists)
		})
	}
}

func TestFileSystem_MkdirAll_Idempotent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			dirPath := "testdir"
			err := fs.MkdirAll(dirPath, 0o755)
			require.NoError(t, err)

			// Call again - should not error
			err = fs.MkdirAll(dirPath, 0o755)
			require.NoError(t, err)
		})
	}
}

func TestFileSystem_Symlink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupFS func(t *testing.T) utils.FileSystem
		name    string
	}{
		{
			name:    "TempFS",
			setupFS: setupTempFS,
		},
		// Note: MemMapFs doesn't support symlinks, so we skip it
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			targetPath := "target.txt"
			linkPath := "link.txt"

			err := fs.WriteFile(targetPath, []byte("target content"), 0o644)
			require.NoError(t, err)

			err = fs.Symlink(targetPath, linkPath)
			require.NoError(t, err)

			// Read through symlink
			content, err := fs.ReadFile(linkPath)
			require.NoError(t, err)
			assert.Equal(t, []byte("target content"), content)
		})
	}
}

func TestFileSystem_Symlink_NoSupport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupFS func(t *testing.T) utils.FileSystem
		name    string
	}{
		// Note: MemMapFs doesn't support symlinks, so we skip it
		{
			name:    "MemFS",
			setupFS: setupMemFS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			targetPath := "target.txt"
			linkPath := "link.txt"

			err := fs.WriteFile(targetPath, []byte("target content"), 0o644)
			require.NoError(t, err)

			err = fs.Symlink(targetPath, linkPath)
			require.Error(t, err)
		})
	}
}

func TestFileSystem_ReadDir(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			// Create directory with files
			err := fs.MkdirAll("testdir", 0o755)
			require.NoError(t, err)

			err = fs.WriteFile("testdir/file1.txt", []byte("test1"), 0o644)
			require.NoError(t, err)

			err = fs.WriteFile("testdir/file2.txt", []byte("test2"), 0o644)
			require.NoError(t, err)

			err = fs.MkdirAll("testdir/subdir", 0o755)
			require.NoError(t, err)

			// Read directory
			entries, err := fs.ReadDir("testdir")
			require.NoError(t, err)
			assert.Len(t, entries, 3)

			names := make([]string, len(entries))
			for i, entry := range entries {
				names[i] = entry.Name()
			}
			assert.Contains(t, names, "file1.txt")
			assert.Contains(t, names, "file2.txt")
			assert.Contains(t, names, "subdir")
		})
	}
}

func TestFileSystem_ReadDir_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			_, err := fs.ReadDir("nonexistent")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_Exists(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testPath := "test.txt"
			err := fs.WriteFile(testPath, []byte("test"), 0o644)
			require.NoError(t, err)

			exists, err := fs.Exists(testPath)
			require.NoError(t, err)
			assert.True(t, exists)

			exists, err = fs.Exists("nonexistent.txt")
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestFileSystem_DirExists(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			dirPath := "testdir"
			err := fs.MkdirAll(dirPath, 0o755)
			require.NoError(t, err)

			exists, err := fs.DirExists(dirPath)
			require.NoError(t, err)
			assert.True(t, exists)

			exists, err = fs.DirExists("nonexistent")
			require.NoError(t, err)
			assert.False(t, exists)

			// Create a file, not a directory
			filePath := "file.txt"
			err = fs.WriteFile(filePath, []byte("test"), 0o644)
			require.NoError(t, err)

			exists, err = fs.DirExists(filePath)
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestFileSystem_Pipe(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := "line1\nline2\nline3\n"

			err := fs.WriteFile(basicTestPath, []byte(testData), 0o644)
			require.NoError(t, err)

			pipe := fs.Pipe(basicTestPath)
			require.NoError(t, pipe.Error())

			content, err := pipe.String()
			require.NoError(t, err)
			assert.Equal(t, testData, content)
		})
	}
}

func TestFileSystem_Pipe_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			pipe := fs.Pipe("nonexistent.txt")
			assert.Error(t, pipe.Error())
		})
	}
}

func TestFileSystem_WritePipe(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := "test content\n"
			pipe := script.Echo(testData)

			testPath := "output.txt"
			n, err := fs.WritePipe(testPath, pipe, os.O_CREATE|os.O_WRONLY, 0o644)
			require.NoError(t, err)
			assert.Equal(t, n, int64(len(testData)))

			content, err := fs.ReadFile(testPath)
			require.NoError(t, err)
			assert.Equal(t, testData, string(content))
		})
	}
}

func TestFileSystem_WritePipe_WithError(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			// Create a pipe with an error
			pipe := script.NewPipe().WithError(assert.AnError)

			testPath := "output.txt"
			_, err := fs.WritePipe(testPath, pipe, os.O_CREATE|os.O_WRONLY, 0o644)
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_WritePipe_NonExistentDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupFS func(t *testing.T) utils.FileSystem
		name    string
	}{
		{
			name:    "TempFS",
			setupFS: setupTempFS,
		},
		// Don't test MemFS here because it auto-creates directories
		// {
		// 	name:    "MemFS",
		// 	setupFS: setupMemFS,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			testData := "test content\n"
			pipe := script.Echo(testData)

			testPath := "nonexistent/output.txt"
			_, err := fs.WritePipe(testPath, pipe, os.O_CREATE|os.O_WRONLY, 0o644)
			assert.Error(t, err)
		})
	}
}

type faultyReader struct{}

func (f *faultyReader) Read(_ []byte) (int, error) {
	return 0, assert.AnError
}

func TestFileSystem_WritePipe_CopyError(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			pipe := script.NewPipe().WithReader(&faultyReader{})

			testPath := "nonexistent/output.txt"
			_, err := fs.WritePipe(testPath, pipe, os.O_CREATE|os.O_WRONLY, 0o644)
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_Rename(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			oldPath := "old.txt"
			newPath := "new.txt"
			testData := []byte("test content")

			err := fs.WriteFile(oldPath, testData, 0o644)
			require.NoError(t, err)

			err = fs.Rename(oldPath, newPath)
			require.NoError(t, err)

			// Old file should not exist
			exists, err := fs.Exists(oldPath)
			require.NoError(t, err)
			assert.False(t, exists)

			// New file should exist with same content
			content, err := fs.ReadFile(newPath)
			require.NoError(t, err)
			assert.Equal(t, testData, content)
		})
	}
}

func TestFileSystem_Rename_NonExistent(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			err := fs.Rename("nonexistent.txt", "new.txt")
			assert.Error(t, err)
		})
	}
}

func TestFileSystem_Integration_ComplexWorkflow(t *testing.T) {
	t.Parallel()

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := tt.setupFS(t)

			// Create directory structure
			err := fs.MkdirAll("project/src", 0o755)
			require.NoError(t, err)

			err = fs.MkdirAll("project/tests", 0o755)
			require.NoError(t, err)

			// Create files
			err = fs.WriteFile("project/src/main.go", []byte("package main"), 0o644)
			require.NoError(t, err)

			err = fs.WriteFile("project/tests/test.go", []byte("package tests"), 0o644)
			require.NoError(t, err)

			// List files
			entries, err := fs.ReadDir("project")
			require.NoError(t, err)
			assert.Len(t, entries, 2)

			// Read and modify file
			content, err := fs.ReadFile("project/src/main.go")
			require.NoError(t, err)

			content = append(content, []byte("\n\nfunc main() {}")...)
			err = fs.WriteFile("project/src/main.go", content, 0o644)
			require.NoError(t, err)

			// Verify modification
			content, err = fs.ReadFile("project/src/main.go")
			require.NoError(t, err)
			assert.Contains(t, string(content), "func main()")

			// Move file
			err = fs.Rename("project/tests/test.go", "project/src/test.go")
			require.NoError(t, err)

			exists, err := fs.Exists("project/src/test.go")
			require.NoError(t, err)
			assert.True(t, exists)

			exists, err = fs.Exists("project/tests/test.go")
			require.NoError(t, err)
			assert.False(t, exists)

			// Clean up
			err = fs.RemoveAll("project")
			require.NoError(t, err)

			exists, err = fs.Exists("project")
			require.NoError(t, err)
			assert.False(t, exists)
		})
	}
}

func TestFileSystem_TempFSWithRealPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	fs := setupTempFSOnDirectory(t, tempDir)

	// Write a file
	testData := []byte("test content")
	err := fs.WriteFile(basicTestPath, testData, 0o644)
	require.NoError(t, err)

	// Verify file exists in temp directory
	realPath := filepath.Join(tempDir, basicTestPath)
	_, err = os.Stat(realPath)
	require.NoError(t, err)

	// Verify content matches
	osContent, err := os.ReadFile(realPath) //nolint:gosec // test code
	require.NoError(t, err)
	assert.Equal(t, testData, osContent)
}

func TestNewMemMapFS(t *testing.T) {
	t.Parallel()

	fs := utils.NewMemMapFS()
	require.NotNil(t, fs)

	// Basic functionality test
	testData := []byte("test")

	err := fs.WriteFile(basicTestPath, testData, 0o644)
	require.NoError(t, err)

	content, err := fs.ReadFile(basicTestPath)
	require.NoError(t, err)
	assert.Equal(t, testData, content)
}
