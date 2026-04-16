// Package host provides host-level filesystem and command execution abstractions.
// This file defines a FileSystem interface that abstracts filesystem operations,
// allowing for both real filesystem access and testing with in-memory filesystems.
//
//nolint:wrapcheck // Justification: Test compatible interface.
package host

// cSpell: words wrapcheck interfacebloat

// cSpell: disable

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bitfield/script"
	"github.com/spf13/afero"
)

// cSpell: enable

// FileSystem defines an interface for filesystem operations.
// It abstracts the underlying filesystem implementation, allowing for easy testing
// with mock or in-memory filesystems while maintaining compatibility with real filesystem operations.
//
//nolint:interfacebloat // Justification: This interface is intended to cover a wide range of filesystem operations.
type FileSystem interface {
	// ReadFile reads the file at the given path and returns its contents.
	// It returns an error if the file does not exist or cannot be read.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to the file at the given path with the specified permissions.
	// It creates the file if it does not exist, or truncates it if it does.
	WriteFile(path string, data []byte, perm os.FileMode) error

	// Stat returns the FileInfo for the file at the given path.
	// It returns an error if the file does not exist or cannot be accessed.
	Stat(path string) (os.FileInfo, error)

	// Create creates or truncates the file at the given path.
	// It returns the opened file or an error if the operation fails.
	Create(path string) (afero.File, error)

	// Open opens the file at the given path for reading.
	// It returns an error if the file does not exist or cannot be opened.
	Open(path string) (afero.File, error)

	// OpenFile opens the file at the given path with the specified flags and permissions.
	// It returns the opened file or an error if the operation fails.
	OpenFile(path string, flag int, perm os.FileMode) (afero.File, error)

	// Remove deletes the file at the given path.
	// It returns an error if the file does not exist or cannot be deleted.
	Remove(path string) error

	// RemoveAll deletes the file or directory at the given path and all its contents.
	// It returns an error if the operation fails.
	RemoveAll(path string) error

	// MkdirAll creates a directory along with any necessary parent directories.
	// It does nothing if the directory already exists.
	MkdirAll(path string, perm os.FileMode) error

	// Symlink creates a symbolic link at newName pointing to oldName.
	// It returns an error if the operation fails or is not supported by the filesystem.
	Symlink(oldName, newName string) error

	// ReadDir reads the directory at the given path and returns a list of file entries.
	// It returns an error if the directory does not exist or cannot be read.
	ReadDir(dirname string) ([]os.FileInfo, error)

	// Exists checks if the file or directory at the given path exists.
	// It returns true if the path exists, false otherwise. An error is returned if the check fails.
	Exists(path string) (bool, error)

	// Pipe creates a script.Pipe for the file at the given path.
	// It returns the pipe for further processing or an error if the file cannot be opened.
	Pipe(path string) *script.Pipe

	// WritePipe writes the contents of the given script.Pipe to the file at the specified path.
	// It returns the number of bytes written and an error if the operation fails.
	WritePipe(path string, pipe *script.Pipe, flag int, perm os.FileMode) (int64, error)

	// DirExists checks if the directory at the given path exists.
	DirExists(path string) (bool, error)

	// Rename moves a file from oldPath to newPath.
	Rename(oldPath, newPath string) error

	// EvalSymlinks evaluates symbolic links in the given path and returns the actual path.
	EvalSymlinks(path string) (string, error)

	// Glob returns the list of matching files for the given pattern.
	Glob(pattern string) (matches []string, err error)

	// Walk walks the file tree rooted at root, calling walkFn for each file or directory in the tree, including root.
	Walk(root string, walkFn filepath.WalkFunc) error

	// Abs returns the absolute path for the given path, resolving any symbolic links.
	Abs(path string) (string, error)
}

type hostImpl struct {
	fs afero.Fs
}

var _ FileSystem = (*hostImpl)(nil)

func (a *hostImpl) ReadFile(path string) ([]byte, error) {
	return afero.ReadFile(a.fs, path)
}

func (a *hostImpl) WriteFile(path string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(a.fs, path, data, perm)
}

func (a *hostImpl) Stat(path string) (os.FileInfo, error) {
	return a.fs.Stat(path)
}

func (a *hostImpl) Create(path string) (afero.File, error) {
	return a.fs.Create(path)
}

func (a *hostImpl) Open(path string) (afero.File, error) {
	return a.fs.Open(path)
}

func (a *hostImpl) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	return a.fs.OpenFile(path, flag, perm)
}

func (a *hostImpl) Remove(path string) error {
	return a.fs.Remove(path)
}

func (a *hostImpl) RemoveAll(path string) error {
	return a.fs.RemoveAll(path)
}

func (a *hostImpl) MkdirAll(path string, perm os.FileMode) error {
	return a.fs.MkdirAll(path, perm)
}

// Symlink creates a symbolic link at newName pointing to oldName.
// It returns an error if the underlying filesystem does not support symlinks.
func (a *hostImpl) Symlink(oldName, newName string) error {
	linker, ok := a.fs.(afero.Linker)
	if !ok { // coverage-ignore
		return fmt.Errorf("filesystem does not support symlinks")
	}
	return linker.SymlinkIfPossible(oldName, newName)
}

func (a *hostImpl) ReadDir(dirname string) ([]os.FileInfo, error) {
	return afero.ReadDir(a.fs, dirname)
}

func (a *hostImpl) Exists(path string) (bool, error) {
	return afero.Exists(a.fs, path)
}

func (a *hostImpl) Pipe(path string) *script.Pipe {
	result := script.NewPipe()
	file, err := a.Open(path)
	if err != nil {
		return result.WithError(fmt.Errorf("while opening %s: %w", path, err))
	}

	return result.WithReader(file)
}

func (a *hostImpl) WritePipe(
	path string,
	p *script.Pipe,
	flag int,
	perm os.FileMode,
) (int64, error) {
	if p.Error() != nil {
		return 0, p.Error()
	}
	out, err := a.OpenFile(path, flag, perm)
	if err != nil {
		p.SetError(err)
		return 0, err
	}
	defer func() {
		err = out.Close()
	}()

	wrote, err := io.Copy(out, p)
	if err != nil {
		p.SetError(err)
	}
	return wrote, p.Error()
}

func (a *hostImpl) DirExists(path string) (bool, error) {
	return afero.DirExists(a.fs, path)
}

func (a *hostImpl) Rename(oldPath, newPath string) error {
	return a.fs.Rename(oldPath, newPath)
}

func (a *hostImpl) EvalSymlinks(path string) (string, error) {
	_, ok := a.fs.(*afero.OsFs)
	if ok {
		// If it's an OsFs, we can use the standard library's EvalSymlinks which is more robust.
		return filepath.EvalSymlinks(path)
	}
	evalLinker, ok := a.fs.(afero.LinkReader)
	if !ok {
		return "", fmt.Errorf("filesystem does not support evaluating symlinks")
	}
	return evalLinker.ReadlinkIfPossible(path) //nolint:wrapcheck // preserve the original error type.
}

func (a *hostImpl) Glob(pattern string) ([]string, error) {
	return afero.Glob(a.fs, pattern)
}

func (a *hostImpl) Walk(root string, walkFn filepath.WalkFunc) error {
	return afero.Walk(a.fs, root, walkFn)
}

func (a *hostImpl) Abs(path string) (string, error) {
	aBase := afero.NewBasePathFs(a.fs, string([]rune{filepath.Separator}))
	base, ok := aBase.(*afero.BasePathFs)
	if !ok {
		return "", fmt.Errorf("failed to assert BasePathFs")
	}

	return base.RealPath(path)
}

// NewMemMapFS creates in-memory filesystem for tests.
func NewMemMapFS() FileSystem {
	return &hostImpl{fs: afero.NewMemMapFs()}
}

// NewBasePathFS creates a FileSystem that is rooted at the given basePath.
// All file operations will be relative to this base path.
func NewBasePathFS(baseFS afero.Fs, basePath string) FileSystem {
	return &hostImpl{fs: afero.NewBasePathFs(baseFS, basePath)}
}

func NewOsFS() FileSystem {
	return &hostImpl{fs: afero.NewOsFs()}
}
