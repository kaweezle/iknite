// Package utils provides utility functions and interfaces for filesystem operations.
// This file defines a FileSystem interface that abstracts filesystem operations,
// allowing for both real filesystem access and testing with in-memory filesystems.
//
//nolint:wrapcheck // Justification: Test compatible interface.
package utils

// cSpell: words wrapcheck interfacebloat

// cSpell: disable

import (
	"fmt"
	"io"
	"os"

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

	// MoveFile moves a file from oldPath to newPath.
	Rename(oldPath, newPath string) error
}

// aferoFS is a concrete implementation of the FileSystem interface using afero.Fs.
// It wraps an afero filesystem backend and delegates all operations to it.
type aferoFS struct {
	fs afero.Fs
}

// FS is the global FileSystem instance used throughout the application.
// It defaults to a real filesystem (NewOsFs) but can be swapped for testing purposes.
var FS FileSystem = &aferoFS{fs: afero.NewOsFs()}

func (a *aferoFS) ReadFile(path string) ([]byte, error) {
	return afero.ReadFile(a.fs, path)
}

func (a *aferoFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(a.fs, path, data, perm)
}

func (a *aferoFS) Stat(path string) (os.FileInfo, error) {
	return a.fs.Stat(path)
}

func (a *aferoFS) Create(path string) (afero.File, error) {
	return a.fs.Create(path)
}

func (a *aferoFS) Open(path string) (afero.File, error) {
	return a.fs.Open(path)
}

func (a *aferoFS) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	return a.fs.OpenFile(path, flag, perm)
}

func (a *aferoFS) Remove(path string) error {
	return a.fs.Remove(path)
}

func (a *aferoFS) RemoveAll(path string) error {
	return a.fs.RemoveAll(path)
}

func (a *aferoFS) MkdirAll(path string, perm os.FileMode) error {
	return a.fs.MkdirAll(path, perm)
}

// Symlink creates a symbolic link at newName pointing to oldName.
// It returns an error if the underlying filesystem does not support symlinks.
func (a *aferoFS) Symlink(oldName, newName string) error {
	linker, ok := a.fs.(afero.Linker)
	if !ok { // coverage-ignore
		return fmt.Errorf("filesystem does not support symlinks")
	}
	return linker.SymlinkIfPossible(oldName, newName)
}

func (a *aferoFS) ReadDir(dirname string) ([]os.FileInfo, error) {
	return afero.ReadDir(a.fs, dirname)
}

func (a *aferoFS) Exists(path string) (bool, error) {
	return afero.Exists(a.fs, path)
}

func (a *aferoFS) Pipe(path string) *script.Pipe {
	result := script.NewPipe()
	file, err := a.Open(path)
	if err != nil {
		return result.WithError(fmt.Errorf("while opening %s: %w", path, err))
	}

	return result.WithReader(file)
}

func (a *aferoFS) WritePipe(
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

func (a *aferoFS) DirExists(path string) (bool, error) {
	return afero.DirExists(a.fs, path)
}

func (a *aferoFS) Rename(oldPath, newPath string) error {
	return a.fs.Rename(oldPath, newPath)
}

// For tests.
func NewMemMapFS() FileSystem {
	return &aferoFS{fs: afero.NewMemMapFs()}
}

// NewBasePathFS creates a FileSystem that is rooted at the given basePath.
// All file operations will be relative to this base path.
func NewBasePathFS(baseFS afero.Fs, basePath string) FileSystem {
	return &aferoFS{fs: afero.NewBasePathFs(baseFS, basePath)}
}
