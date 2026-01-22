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
	"context"
	"fmt"
	"net"
	"time"
)

// ExecuteOnExistence executes the function fn if the file existence is the
// one given by the parameter.
func ExecuteOnExistence(file string, existence bool, fn func() error) error {
	exists, err := FS.Exists(file)
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
func ExecuteIfNotExist(file string, fn func() error) error {
	return ExecuteOnExistence(file, false, fn)
}

// ExecuteIfExist executes the function fn if the file
// exists.
func ExecuteIfExist(file string, fn func() error) error {
	return ExecuteOnExistence(file, true, fn)
}

// MoveFileIfExists moves the file src to the destination dst
// if it exists.
func MoveFileIfExists(src, dst string) error {
	exists, err := FS.Exists(src)
	if err != nil {
		return fmt.Errorf("error while checking existence of %s: %w", src, err)
	}
	if !exists {
		return nil
	}

	if err := FS.Rename(src, dst); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %w", src, dst, err)
	}
	return nil
}

// GetOutboundIP returns the preferred outbound ip of this machine.
func GetOutboundIP() (net.IP, error) {
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "udp", "8.8.8.8:80")
	if err != nil {
		return nil, fmt.Errorf("error while getting IP address: %w", err)
	}
	defer func() {
		err = conn.Close()
	}()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("failed to get local address")
	}

	return localAddr.IP, nil
}

func IsOnWSL() bool {
	wsl, err := FS.DirExists("/run/WSL")
	if err != nil {
		return false
	}
	return wsl
}
