package host

import (
	"fmt"
	"path/filepath"
)

func IsOnWSL(fs FileSystem) bool {
	wsl, err := fs.DirExists("/run/WSL")
	if err != nil {
		return false
	}
	return wsl
}

func IsOnIncus(fs FileSystem) bool {
	incus, err := fs.DirExists("/dev/.lxc/proc")
	if err != nil {
		return false
	}
	return incus
}

// ExecuteOnExistence executes the function fn if the file existence is the
// one given by the parameter.
func ExecuteOnExistence(fs FileSystem, file string, existence bool, fn func() error) error {
	exists, err := fs.Exists(file)
	if err != nil {
		return fmt.Errorf("error while checking if %s exists: %w", file, err)
	}

	if exists == existence {
		return fn()
	}
	return nil
}

// ExecuteIfNotExist executes the function fn if the file
// doesn't exist.
func ExecuteIfNotExist(fs FileSystem, file string, fn func() error) error {
	return ExecuteOnExistence(fs, file, false, fn)
}

// ExecuteIfExist executes the function fn if the file
// exists.
func ExecuteIfExist(fs FileSystem, file string, fn func() error) error {
	return ExecuteOnExistence(fs, file, true, fn)
}

// MoveFileIfExists moves the file src to the destination dst
// if it exists.
func MoveFileIfExists(fs FileSystem, src, dst string) error {
	exists, err := fs.Exists(src)
	if err != nil {
		return fmt.Errorf("error while checking existence of %s: %w", src, err)
	}
	if !exists {
		return nil
	}

	if err := fs.Rename(src, dst); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %w", src, dst, err)
	}
	return nil
}

// CleanDir removes everything in a directory, but not the directory itself.
func CleanDir(fs FileSystem, dir string) error {
	exists, err := fs.DirExists(dir)
	if err != nil {
		return fmt.Errorf("error while checking existence of directory %s: %w", dir, err)
	}
	if !exists {
		return nil
	}

	files, err := fs.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("error while reading directory %s: %w", dir, err)
	}
	for _, file := range files {
		if err := fs.RemoveAll(filepath.Join(dir, file.Name())); err != nil {
			return fmt.Errorf("error while removing file %s: %w", filepath.Join(dir, file.Name()), err)
		}
	}
	return nil
}
