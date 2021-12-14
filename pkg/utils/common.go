package utils

import (
	"net"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

// ExecuteIfNotExist executes the function fn if the file file
// doesn't exist.
func ExecuteIfNotExist(file string, fn func() error) error {
	exists, err := afs.Exists(file)
	if err != nil {
		return errors.Wrapf(err, "Error while checking if %s exists", file)
	}

	if !exists {
		return fn()
	}
	return nil
}

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
