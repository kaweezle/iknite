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
	KubectlCmd                    = "/usr/bin/kubectl"
	CrioSock                      = "/run/crio/crio.sock"
	CrioServiceName               = "crio"
	KubeletServiceName            = "kubelet"
	MDNSServiceName               = "iknite-mdns"
	ConfigureServiceName          = "iknite-config"
	KubernetesAdminConfig         = "/etc/kubernetes/admin.conf"
	KubernetesRootConfig          = "/root/.kube/config"
	DefaultClusterName            = "kaweezle"
	DefaultKustomizationDirectory = "/etc/iknite.d"
	WSLHostName                   = "kaweezle.local"
	WSLIPAddress                  = "192.168.99.2"
)
