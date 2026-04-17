package host_test

// cSpell: words kyaml wrapcheck testdir iface mydir mypath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestKustomizeFSWrapper_Create(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)

	f, err := kfs.Create("test.txt")
	require.NoError(t, err)
	require.NotNil(t, f)
	f.Close() //nolint:errcheck // best-effort close in test
}

func TestKustomizeFSWrapper_Mkdir(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)

	err := kfs.Mkdir("testdir")
	require.NoError(t, err)
	assert.True(t, kfs.IsDir("testdir"))
}

func TestKustomizeFSWrapper_MkdirAll(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)

	err := kfs.MkdirAll("dir1/dir2/dir3")
	require.NoError(t, err)
	assert.True(t, kfs.IsDir("dir1/dir2/dir3"))
}

func TestKustomizeFSWrapper_Open(t *testing.T) {
	t.Parallel()

	t.Run("existing file", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, kfs.WriteFile("test.txt", []byte("content")))

		f, err := kfs.Open("test.txt")
		require.NoError(t, err)
		defer f.Close() //nolint:errcheck // ignore close error in test
	})

	t.Run("non-existent file", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)

		_, err := kfs.Open("nonexistent.txt")
		assert.Error(t, err)
	})
}

func TestKustomizeFSWrapper_OpenFile(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfsIface := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfsIface.WriteFile("test.txt", []byte("content")))

	kfs, ok := kfsIface.(*host.KustomizeFSWrapper)
	require.True(t, ok)

	f, err := kfs.OpenFile("test.txt", os.O_RDONLY, 0o644)
	require.NoError(t, err)
	require.NotNil(t, f)
	f.Close() //nolint:errcheck // best-effort close in test
}

func TestKustomizeFSWrapper_Remove(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfsIface := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfsIface.WriteFile("test.txt", []byte("content")))

	kfs, ok := kfsIface.(*host.KustomizeFSWrapper)
	require.True(t, ok)

	err := kfs.Remove("test.txt")
	require.NoError(t, err)
	assert.False(t, kfsIface.Exists("test.txt"))
}

func TestKustomizeFSWrapper_RemoveAll(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfs.MkdirAll("dir1/dir2"))
	require.NoError(t, kfs.WriteFile("dir1/file.txt", []byte("content")))

	err := kfs.RemoveAll("dir1")
	require.NoError(t, err)
	assert.False(t, kfs.IsDir("dir1"))
}

func TestKustomizeFSWrapper_ReadDir(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, kfs.MkdirAll("testdir"))
		require.NoError(t, kfs.WriteFile("testdir/a.txt", []byte("a")))
		require.NoError(t, kfs.WriteFile("testdir/b.txt", []byte("b")))

		entries, err := kfs.ReadDir("testdir")
		require.NoError(t, err)
		assert.Len(t, entries, 2)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)

		_, err := kfs.ReadDir("nonexistent")
		assert.Error(t, err)
	})
}

func TestKustomizeFSWrapper_Exists(t *testing.T) {
	t.Parallel()

	t.Run("exists", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, kfs.WriteFile("test.txt", []byte("content")))

		assert.True(t, kfs.Exists("test.txt"))
	})

	t.Run("not exists", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)

		assert.False(t, kfs.Exists("test.txt"))
	})

	t.Run("error returns false", func(t *testing.T) {
		t.Parallel()
		mockFS := host.NewMockFileSystem(t)
		mockFS.On("Exists", "test.txt").Return(false, assert.AnError).Once()

		kfs := host.NewKustomizeFSWrapper(mockFS)
		assert.False(t, kfs.Exists("test.txt"))
	})
}

func TestKustomizeFSWrapper_WriteFile(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)

	err := kfs.WriteFile("test.txt", []byte("hello"))
	require.NoError(t, err)

	content, err := kfs.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), content)
}

func TestKustomizeFSWrapper_Symlink(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fs := host.NewBasePathFS(afero.NewOsFs(), tempDir)
	kfsIface := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfsIface.WriteFile("target.txt", []byte("content")))

	kfs, ok := kfsIface.(*host.KustomizeFSWrapper)
	require.True(t, ok)

	err := kfs.Symlink("target.txt", "link.txt")
	require.NoError(t, err)
}

func TestKustomizeFSWrapper_CleanedAbs(t *testing.T) {
	t.Parallel()

	t.Run("directory path", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, fs.MkdirAll("/mydir", 0o755))

		dir, base, err := kfs.CleanedAbs("/mydir")
		require.NoError(t, err)
		assert.NotEmpty(t, string(dir))
		assert.Empty(t, base)
	})

	t.Run("file path", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, fs.WriteFile("/test.txt", []byte("content"), 0o644))

		dir, base, err := kfs.CleanedAbs("/test.txt")
		require.NoError(t, err)
		assert.NotEmpty(t, string(dir))
		assert.Equal(t, "test.txt", base)
	})

	t.Run("abs error", func(t *testing.T) {
		t.Parallel()
		mockFS := host.NewMockFileSystem(t)
		mockFS.On("Abs", "mypath").Return("", assert.AnError).Once()

		kfs := host.NewKustomizeFSWrapper(mockFS)
		_, _, err := kfs.CleanedAbs("mypath")
		require.Error(t, err)
		require.Contains(t, err.Error(), "while getting absolute path")
	})

	t.Run("dirExists error", func(t *testing.T) {
		t.Parallel()
		mockFS := host.NewMockFileSystem(t)
		mockFS.On("Abs", "mypath").Return("/mypath", nil).Once()
		mockFS.On("DirExists", "/mypath").Return(false, assert.AnError).Once()

		kfs := host.NewKustomizeFSWrapper(mockFS)
		_, _, err := kfs.CleanedAbs("mypath")
		require.Error(t, err)
		require.Contains(t, err.Error(), "while checking if path is a directory")
	})
}

func TestKustomizeFSWrapper_Glob(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfs.WriteFile("a.txt", []byte("a")))
	require.NoError(t, kfs.WriteFile("b.txt", []byte("b")))
	require.NoError(t, kfs.WriteFile("c.log", []byte("c")))

	matches, err := kfs.Glob("*.txt")
	require.NoError(t, err)
	assert.Len(t, matches, 2)
}

func TestKustomizeFSWrapper_IsDir(t *testing.T) {
	t.Parallel()

	t.Run("is a directory", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, fs.MkdirAll("testdir", 0o755))

		assert.True(t, kfs.IsDir("testdir"))
	})

	t.Run("is a file", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)
		require.NoError(t, fs.WriteFile("test.txt", []byte("content"), 0o644))

		assert.False(t, kfs.IsDir("test.txt"))
	})

	t.Run("non-existent", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		kfs := host.NewKustomizeFSWrapper(fs)

		assert.False(t, kfs.IsDir("nonexistent"))
	})

	t.Run("error returns false", func(t *testing.T) {
		t.Parallel()
		mockFS := host.NewMockFileSystem(t)
		mockFS.On("DirExists", "mydir").Return(false, assert.AnError).Once()

		kfs := host.NewKustomizeFSWrapper(mockFS)
		assert.False(t, kfs.IsDir("mydir"))
	})
}

func TestKustomizeFSWrapper_ReadFile(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, kfs.WriteFile("test.txt", []byte("hello")))

	content, err := kfs.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), content)
}

func TestKustomizeFSWrapper_Walk(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)
	require.NoError(t, fs.MkdirAll("walk/sub", 0o755))
	require.NoError(t, fs.WriteFile("walk/file.txt", []byte("content"), 0o644))

	var walked []string
	err := kfs.Walk("walk", func(path string, _ os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		walked = append(walked, filepath.Base(path))
		return nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, walked)
}

func TestNewKustomizeFSWrapper(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	kfs := host.NewKustomizeFSWrapper(fs)
	require.NotNil(t, kfs)
}
