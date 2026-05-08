// cSpell: words filesys kyaml wrapcheck
//
//nolint:wrapcheck // Justification: Thin wrapper around host.FileSystem to adapt to kustomize's FileSystem interface.
package host

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type KustomizeFSWrapper struct {
	fs FileSystem
}

var _ filesys.FileSystem = (*KustomizeFSWrapper)(nil)

func NewKustomizeFSWrapper(fs FileSystem) filesys.FileSystem {
	return &KustomizeFSWrapper{fs: fs}
}

func (w *KustomizeFSWrapper) Create(path string) (filesys.File, error) {
	return w.fs.Create(path)
}

func (w *KustomizeFSWrapper) Mkdir(name string) error {
	return w.fs.MkdirAll(name, 0o755)
}

func (w *KustomizeFSWrapper) MkdirAll(name string) error {
	return w.fs.MkdirAll(name, 0o755)
}

func (w *KustomizeFSWrapper) Open(path string) (filesys.File, error) {
	return w.fs.Open(path)
}

func (w *KustomizeFSWrapper) OpenFile(name string, flag int, perm os.FileMode) (filesys.File, error) {
	return w.fs.OpenFile(name, flag, perm)
}

func (w *KustomizeFSWrapper) Remove(path string) error {
	return w.fs.Remove(path)
}

func (w *KustomizeFSWrapper) RemoveAll(path string) error {
	return w.fs.RemoveAll(path)
}

func (w *KustomizeFSWrapper) ReadDir(dirname string) ([]string, error) {
	infos, err := w.fs.ReadDir(dirname)
	if err != nil {
		return nil, fmt.Errorf("while getting %s contents: %w", dirname, err)
	}
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name()
	}
	return names, nil
}

func (w *KustomizeFSWrapper) Exists(path string) bool {
	result, err := w.fs.Exists(path)
	if err != nil {
		return false
	}
	return result
}

func (w *KustomizeFSWrapper) WriteFile(path string, data []byte) error {
	return w.fs.WriteFile(path, data, 0o644)
}

func (w *KustomizeFSWrapper) Symlink(oldName, newName string) error {
	return w.fs.Symlink(oldName, newName)
}

func (w *KustomizeFSWrapper) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	actualPath, err := w.fs.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("while getting absolute path: %w", err)
	}

	isDir, err := w.fs.DirExists(actualPath)
	if err != nil {
		return "", "", fmt.Errorf("while checking if path is a directory: %w", err)
	}
	if isDir {
		return filesys.ConfirmedDir(actualPath), "", nil
	}
	dir := filepath.Dir(actualPath)
	base := filepath.Base(actualPath)

	return filesys.ConfirmedDir(dir), base, nil
}

// Glob implements [filesys.FileSystem].
func (w *KustomizeFSWrapper) Glob(pattern string) ([]string, error) {
	return w.fs.Glob(pattern)
}

// IsDir implements [filesys.FileSystem].
func (w *KustomizeFSWrapper) IsDir(path string) bool {
	ok, err := w.fs.DirExists(path)
	if err != nil {
		return false
	}
	return ok
}

// ReadFile implements [filesys.FileSystem].
func (w *KustomizeFSWrapper) ReadFile(path string) ([]byte, error) {
	return w.fs.ReadFile(path)
}

// Walk implements [filesys.FileSystem].
func (w *KustomizeFSWrapper) Walk(path string, walkFn filepath.WalkFunc) error {
	return w.fs.Walk(path, walkFn)
}
