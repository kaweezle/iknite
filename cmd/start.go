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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"net"
	"os"
	"path"
	"time"

	c "github.com/antoinemartin/k8wsl/pkg/constants"
	"github.com/antoinemartin/k8wsl/pkg/provision"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
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
	Run: perform,
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
const crioSock = "/run/crio/crio.sock"

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

// ExecuteIfServiceNotStarted executes the function fn if the service serviceName
// is not started.
func ExecuteIfServiceNotStarted(serviceName string, fn func() error) error {
	serviceLink := path.Join(startedServicesDir, serviceName)
	exists, err := afs.Exists(serviceLink)
	if err != nil {
		return errors.Wrapf(err, "Error while checking if service %s exists", serviceLink)
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
			return errors.Wrapf(err, "Error while starting service %s", serviceName)
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

// RenameConfig changes the name of the cluster and the context from the
// default (kubernetes) to newName in c.
func RenameConfig(c *api.Config, newName string) *api.Config {
	newClusters := make(map[string]*api.Cluster)
	for _, v := range c.Clusters {
		newClusters[newName] = v
	}
	c.Clusters = newClusters

	newContexts := make(map[string]*api.Context)
	for _, v := range c.Contexts {
		newContexts[newName] = v
		v.Cluster = newName
	}
	c.Contexts = newContexts

	c.CurrentContext = newName
	return c
}

type CRIOCondition struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type CRIOStatus struct {
	Conditions []CRIOCondition `json:"conditions"`
}

type CRIOStatusResponse struct {
	Status CRIOStatus `json:"status"`
}

func WaitForCrio() (bool, error) {
	retries := 3
	for retries > 0 {
		exist, err := afs.Exists(crioSock)
		if err != nil {
			return false, errors.Wrapf(err, "Error while checking crio sock %s", crioSock)
		}
		if exist {
			out, err := exec.Command("/usr/bin/crictl", "--runtime-endpoint", "unix:///run/crio/crio.sock", "info").Output()
			if err == nil {
				log.Trace(string(out))
				response := &CRIOStatusResponse{}
				err = json.Unmarshal(out, &response)
				if err == nil {
					conditions := 0
					falseConditions := 0
					for _, v := range response.Status.Conditions {
						conditions += 1
						if !v.Status {
							falseConditions += 1
						}
					}
					if conditions >= 2 && falseConditions == 0 {
						break
					}
				} else {
					log.WithError(err).Warn("Error while parsing crio status")
				}
			} else {
				log.WithError(err).Warn("Error while checking crio sock")
			}
		}
		retries = retries - 1

		log.Debug("Waiting 2 seconds...")
		time.Sleep(2 * time.Second)
	}
	return retries > 0, nil
}

func perform(cmd *cobra.Command, args []string) {

	// Start openrc. Actually it is not used but is requested by
	// the `kubeadm init phase preflight` command
	err := ExecuteIfNotExist(openRCDirectory, func() error {
		if out, err := exec.Command("/sbin/openrc", "-n").CombinedOutput(); err == nil {
			fmt.Println(string(out))
			return nil
		} else {
			return errors.Wrap(err, "Error while starting openrc")
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

	// Allow forwarding
	afs.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), os.FileMode(int(0644)))

	// We need to start CRI-O
	cobra.CheckErr(StartService("crio"))
	available, err := WaitForCrio()
	cobra.CheckErr(err)
	if !available {
		log.Fatal("CRI-O not available")
	}

	ip, err := GetOutboundIP()
	cobra.CheckErr(errors.Wrap(err, "While getting IP address"))

	// If the cluster has not been initialized yet, do it
	exist, err := afs.Exists(kubeletConfigFilename)
	cobra.CheckErr(err)
	if !exist {
		parameters := []string{
			"init",
			fmt.Sprintf("--apiserver-advertise-address=%v", ip),
			"--kubernetes-version=1.22.4",
			"--pod-network-cidr=10.244.0.0/16",
		}
		log.Info("Running", "/usr/bin/kubeadm", strings.Join(parameters, " "), "...")
		if out, err := exec.Command("/usr/bin/kubeadm", parameters...).CombinedOutput(); err != nil {
			log.Fatal(err, "\n", string(out))
		} else {
			log.Trace(string(out))
		}
	} else {
		// Just start the service
		cobra.CheckErr(StartService("kubelet"))
	}

	// TODO: Check that cluster is Ok
	config, err := clientcmd.LoadFromFile("/etc/kubernetes/admin.conf")
	cobra.CheckErr(err)
	cobra.CheckErr(clientcmd.WriteToFile(*RenameConfig(config, "k8wsl"), "/root/.kube/config"))

	// Untaint master. It needs a valid kubeconfig
	if out, err := exec.Command(c.KubectlCmd, "taint", "nodes", "--all", "node-role.kubernetes.io/master-").CombinedOutput(); err != nil {
		if strout := string(out); strout != "error: taint \"node-role.kubernetes.io/master\" not found\n" {
			log.Fatal(err, strout)
		}
	}

	// Apply base customization. This adds the following to the cluster
	// - MetalLB
	// - Flannel
	// - Local path provisioner
	// - Metrics server
	// The outbound ip address is needed for MetalLB.
	context := log.Fields{
		"OutboundIP": ip,
	}
	cobra.CheckErr(provision.ApplyBaseKustomizations(context))

	log.Info("executed")
}
