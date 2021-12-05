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
package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"log"
	"net"
	"os"
	"path"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Creates or starts the cluster",
	Long: `Starts the cluster. Performs the following operations:

- Starts OpenRC,
- Starts CRI-O,
- If Kubelet has never been started, execute kubeadm init to provision
  the cluster,
- Allows the use of kubectl from the root account,
- Installs flannel, metal-lb and local-path-provisioner.
`,
	Run: func(cmd *cobra.Command, args []string) {
		perform()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const servicesDir = "/etc/init.d"
const openRCDirectory = "/run/openrc"
const runLevelDir = "/etc/runlevels/default"

var softLevelFile = path.Join(openRCDirectory, "softlevel")
var startedServicesDir = path.Join(openRCDirectory, "started")

const crioServiceName = "crio"
const kubeletServiceName = "kubelet"

const kubeletConfigFilename = "/var/lib/kubelet/config.yaml"

var fs = afero.NewOsFs()
var afs = &afero.Afero{Fs: fs}

// ExecuteIfNotExist executes the function fn if the file file
// doesn't exist.
func ExecuteIfNotExist(file string, fn func() error) error {
	exists, err := afs.Exists(file)
	if err != nil {
		return err
	}

	if !exists {
		return fn()
	}
	return nil
}

// ExecuteIfServiceNotStarted executes the function fn if the service serviceName
// is not started.
func ExecuteIfServiceNotStarted(serviceName string, fn func() error) error {
	serviceLink := path.Join(startedServicesDir, serviceName)
	exists, err := afs.Exists(serviceLink)
	if err != nil {
		return err
	}
	if !exists {
		return fn()
	}

	return nil
}

// EnableService enables the service named serviceName
func EnableService(serviceName string) error {
	serviceFilename := path.Join(servicesDir, serviceName)
	destinationFilename := path.Join(runLevelDir, serviceName)
	return ExecuteIfNotExist(destinationFilename, func() error {
		return os.Symlink(serviceFilename, destinationFilename)
	})
}

// StartService start the serviceName service if it is not already started.
func StartService(serviceName string) error {
	return ExecuteIfServiceNotStarted(serviceName, func() error {
		if out, err := exec.Command("/sbin/rc-service", serviceName, "start").Output(); err == nil {
			fmt.Println(string(out))
			return nil
		} else {
			return err
		}
	})
}

// MoveFileIfExists moves the file src to the destination dst
// if it exists
func MoveFileIfExists(src string, dst string) error {
	err := os.Link(src, dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return os.Remove(src)
}

// Get preferred outbound ip of this machine
func GetOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func perform() {

	// Start openrc. Actually it is not used but is requested by
	// the `kubeadm init phase preflight` command
	err := ExecuteIfNotExist(openRCDirectory, func() error {
		if out, err := exec.Command("/sbin/openrc", "-n").CombinedOutput(); err == nil {
			fmt.Println(string(out))
			return nil
		} else {
			return err
		}
	})
	cobra.CheckErr(err)

	// OpenRC is picky when starting services if it hasn't been started by
	// init. In our case, init is provided by WSL. Creating this file makes
	// OpenRC happy.
	err = ExecuteIfNotExist(softLevelFile, func() error {
		return afs.WriteFile(softLevelFile, []byte{}, os.FileMode(int(0444)))
	})
	cobra.CheckErr(err)

	// Networking is already started so we pretend the service has been run.
	var networkSource = path.Join(servicesDir, "networking")
	var networkDestination = path.Join(openRCDirectory, "started/networking")
	err = ExecuteIfNotExist(networkDestination, func() error {
		return os.Symlink(networkSource, networkDestination)
	})
	cobra.CheckErr(err)

	// Enable CRIO and Kubelet. Kubelet will be started by kubeadm or by us
	cobra.CheckErr(EnableService(crioServiceName))
	cobra.CheckErr(EnableService(kubeletServiceName))

	// We don't mess with IPV6
	cobra.CheckErr(MoveFileIfExists("/etc/cni/net.d/10-crio-bridge.conf", "/etc/cni/net.d/12-crio-bridge.conf"))

	// We need to start CRI-O
	cobra.CheckErr(StartService("crio"))

	// If the cluster has not been initialized yet, do it
	if _, err := os.Stat(kubeletConfigFilename); err != nil {
		if ip, err := GetOutboundIP(); err != nil {
			log.Fatal(err)
		} else {
			parameters := []string{
				"init",
				fmt.Sprintf("--apiserver-advertise-address=%v", ip),
				"--kubernetes-version=1.22.4",
				"--pod-network-cidr 10.244.0.0/16",
			}
			fmt.Println("Running", "/usr/bin/kubeadm", strings.Join(parameters, " "), "...")
			if out, err := exec.Command("/usr/bin/kubeadm", parameters...).Output(); err != nil {
				log.Fatal(err)
			} else {
				fmt.Print(string(out))
			}
			// TODO: Missing
			// Copy kubelet to home directory.
			//  - Add flannel (Kustomize ?)
			//  - Add Metal LB (Kustomize ?)
			//  - Add
			//  - Untaint master
		}
	} else {
		// Just start the service
		cobra.CheckErr(StartService("kubelet"))
	}

	fmt.Println("executed")
}
