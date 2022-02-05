/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

// ExecuteOnExistence executes the function fn if the file existence is the
// one given by the parameter.
func ExecuteOnExistence(file string, existence bool, fn func() error) error {
	exists, err := afs.Exists(file)
	if err != nil {
		return errors.Wrapf(err, "Error while checking if %s exists", file)
	}

	if exists == existence {
		return fn()
	}
	return nil
}

// ExecuteIfNotExist executes the function fn if the file file
// doesn't exist.
func ExecuteIfNotExist(file string, fn func() error) error {
	return ExecuteOnExistence(file, false, fn)
}

// ExecuteIfExist executes the function fn if the file file
// exists.
func ExecuteIfExist(file string, fn func() error) error {
	return ExecuteOnExistence(file, true, fn)
}

// Exists tells if file exists
func Exists(path string) (bool, error) {
	return afs.Exists(path)
}

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return afs.WriteFile(filename, data, perm)
}

// MoveFileIfExists moves the file src to the destination dst
// if it exists
func MoveFileIfExists(src string, dst string) error {
	err := os.Link(src, dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "Error while linking %s to %s", src, dst)
	}

	return os.Remove(src)
}

// GetOutboundIP returns the preferred outbound ip of this machine
func GetOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, errors.Wrap(err, "Error while getting IP address")
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func RemoveDirectoryContents(dir string, predicate func(string) bool) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if predicate == nil || predicate(name) {
			err = os.RemoveAll(filepath.Join(dir, name))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
