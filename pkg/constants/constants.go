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
package constants

const (
	KubectlCmd           = "/usr/bin/kubectl"
	ContainerServiceSock = "/run/containerd/containerd.sock"
	ContainerServiceName = "containerd"
	IkniteService        = "iknite"
	KubernetesRootConfig = "/root/.kube/config"
	DefaultClusterName   = "kaweezle"
	DefaultKustomization = "/etc/iknite.d"
	WSLHostName          = "kaweezle.local"
	WslIPAddress         = "192.168.99.2"
	KubernetesVersion    = "1.32.3"
	NetworkInterface     = "eth0"
	PodSubnet            = "10.244.0.0/16"
	StatusDirectory      = "/run/iknite"
	StatusFile           = "/run/iknite/status.json"
	CrictlYaml           = "/etc/crictl.yaml"
	RcConfFile           = "/etc/rc.conf"
	SoftLevelPath        = "/run/openrc/softlevel" // cSpell: disable-line
)
