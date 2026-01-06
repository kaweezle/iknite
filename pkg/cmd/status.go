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

// cSpell: words runlevels runlevel apiserver controllermanager healthcheck logrus
// cSpell: disable
import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// cSpell: enable

var pkiFiles = []string{
	"etcd/ca.key",
	"etcd/healthcheck-client.key",
	"etcd/peer.key",
	"etcd/ca.crt",
	"etcd/healthcheck-client.crt",
	"etcd/peer.crt",
	"etcd/server.key",
	"etcd/server.crt",
	"sa.pub",
	"front-proxy-ca.key",
	"ca.key",
	"apiserver-kubelet-client.crt",
	"apiserver.crt",
	"apiserver-etcd-client.crt",
	"sa.key",
	"ca.crt",
	"front-proxy-client.crt",
	"front-proxy-client.key",
	"apiserver.key",
	"apiserver-etcd-client.key",
	"apiserver-kubelet-client.key",
	"front-proxy-ca.crt",
}

var (
	timeout      = 0
	checkTimeout = 10 * time.Second
)

func NewStatusCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	// configureCmd represents the start command
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Gives status information on the cluster",
		Long: `Gives status information of the deployed workloads:

- Deployments
- Daemonsets
- Statefulsets
`,
		PersistentPreRun: config.StartPersistentPreRun,
		Run:              func(cmd *cobra.Command, args []string) { performStatus(ikniteConfig) },
	}

	flags := statusCmd.Flags()
	config.ConfigureClusterCommand(flags, ikniteConfig)

	return statusCmd
}

func performStatus(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	var checkData = k8s.CreateCheckWorkloadData(ikniteConfig.GetApiEndPoint())
	var checkDataBuilder = func() k8s.CheckData {
		return checkData
	}

	// Create all checks
	checks := []*k8s.Check{

		// Phase 1: Environment
		k8s.NewPhase("environment", "Environment configuration", []*k8s.Check{
			k8s.SystemFileCheck("ip_forward", "Check IP forwarding is enabled", "/proc/sys/net/ipv4/ip_forward", "1\n"),
			k8s.SystemFileCheck("bridge_nf_call_iptables", "Check IP Tables is active for bridges", "/proc/sys/net/bridge/bridge-nf-call-iptables", "1\n"),
			k8s.SystemFileCheck("machine_id", "Check machine id is defined", "/etc/machine-id", ""),
			k8s.SystemFileCheck("crictl_yaml", "Check crictl configuration is defined", "/etc/crictl.yaml", ""),
			//   - Check if the kubelet service is not runnable
			{
				Name:        "kubelet_service",
				Description: "Check if the kubelet service is not runnable",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					runnable, err := k8s.IsKubeletServiceRunnable(constants.RcConfFile)
					if err != nil {
						return false, "", err
					}
					if runnable {
						return false, "Kubelet service is runnable", nil
					}
					return true, "/etc/rc.conf hack preventing kubelet from running in place", nil
				},
			},
			//   - Check if the iknite service is set to run in default mode
			k8s.SystemFileCheck("iknite_service", "Check if iknite is active on default runlevel", "/etc/runlevels/default/iknite", ""),
			//   - Check if the IP address we are targeting is bound to an interface
			{
				Name:        "ip_bound",
				Description: "Check if the IP address is bound to an interface",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					if ikniteConfig.CreateIp {
						result, err := alpine.CheckIpExists(ikniteConfig.Ip)
						if err != nil {
							return false, "", err
						} else if result {
							return true, fmt.Sprintf("IP address %s is created", ikniteConfig.Ip.String()), nil
						} else {
							return false, fmt.Sprintf("IP address %s is not created", ikniteConfig.Ip.String()), nil
						}
					} else {
						return true, "Don't need to create IP", nil
					}
				},
			},
			//   - Check if the domain name is set
			{
				Name:        "domain_name",
				Description: "Check if the domain name is set",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					if ikniteConfig.DomainName != "" {
						ipString := ikniteConfig.Ip.String()
						if contains, ips := alpine.IsHostMapped(ikniteConfig.Ip, ikniteConfig.DomainName); contains {
							mapped := func() bool {
								for _, ip := range ips {
									if ip.String() == ipString {
										return true
									}
								}
								return false
							}()
							if mapped {
								return true, fmt.Sprintf("Domain name %s is mapped to IP %s", ikniteConfig.DomainName, ipString), nil
							}
						}
						return false, fmt.Sprintf("Domain name %s is not mapped to IP %s", ikniteConfig.DomainName, ipString), nil
					} else {
						return true, "Domain name is not set", nil
					}
				},
			},
		}),

		// Phase 2: Kubernetes configuration
		k8s.NewPhase("configuration", "Kubernetes configuration", []*k8s.Check{
			k8s.FileTreeCheck("pki", "Check PKI files", "/etc/kubernetes/pki", pkiFiles),
			k8s.NewPhase("manifests", "Kubernetes manifests", []*k8s.Check{
				k8s.KubernetesFileCheck("manifest_etcd", "/etc/kubernetes/manifests/etcd.yaml"),
				k8s.KubernetesFileCheck("manifest_apiserver", "/etc/kubernetes/manifests/kube-apiserver.yaml"),
				k8s.KubernetesFileCheck("manifest_controller", "/etc/kubernetes/manifests/kube-controller-manager.yaml"),
				k8s.KubernetesFileCheck("manifest_scheduler", "/etc/kubernetes/manifests/kube-scheduler.yaml"),
			}),
			k8s.KubernetesFileCheck("kubelet_conf", "/etc/kubernetes/kubelet.conf"),
			k8s.KubernetesFileCheck("admin_conf", "/etc/kubernetes/admin.conf"),
			k8s.KubernetesFileCheck("kubelet_config", "/var/lib/kubelet/config.yaml"),
			k8s.KubernetesFileCheck("kubeadm_flags", "/var/lib/kubelet/kubeadm-flags.env"),
			// Check that the etcd data directory is present and contains data
			{
				Name:        "etcd_data",
				Description: "Check that the etcd data directory (/var/lib/etcd) is present and contains data",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					missingFiles, _, err := k8s.FileTreeDifference("/var/lib/etcd", []string{"member/snap/db"})
					if err != nil {
						return false, "", err
					}
					if len(missingFiles) > 0 {
						return false, "/var/lib/etcd has no data file", nil
					}
					return true, "/var/lib/etcd has data files", nil
				},
			},
		}),

		// Phase 3: Runtime status
		k8s.NewPhase("runtime", "Runtime status", []*k8s.Check{
			// Check that openrc is started
			{
				Name:        "openrc",
				Description: "Check that OpenRC is started",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					exists, err := utils.Exists(constants.SoftLevelPath)
					if err != nil {
						return false, "", err
					}
					if !exists {
						return false, "OpenRC is not started", nil
					}
					return true, "OpenRC is started", nil
				},
			},
			k8s.ServiceCheck("iknite_running", "iknite"),
			k8s.ServiceCheck("containerd_running", "containerd"),
			k8s.ServiceCheck("buildkitd_running", "buildkitd"),
			//  - Check if the kubelet process is running
			{
				Name:        "kubelet_running",
				DependsOn:   []string{"iknite_running"},
				Description: "Check if the kubelet process is running",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					return k8s.CheckService("kubelet", false, true)
				},
			},
			//   - Check if the kubelet api endpoint (socket) is reachable and healthy
			{
				Name:        "kubelet_health",
				DependsOn:   []string{"kubelet_running"},
				Description: "Check if the kubelet is reachable and healthy",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					return k8s.CheckKubeletHealth(checkTimeout)
				},
			},
			//   - Check if the kube-apiserver is healthy
			{
				Name:        "apiserver_health",
				DependsOn:   []string{"kubelet_running"},
				Description: "Check if the kube-apiserver is healthy",
				CheckFn: func(ctx context.Context, data k8s.CheckData) (bool, string, error) {
					return k8s.CheckApiServerHealth(checkTimeout, data)
				},
				CheckDataBuilder: checkDataBuilder,
			},
		}),
		{
			Name:             "workload_status",
			Description:      "Check Workload Status",
			DependsOn:        []string{"runtime"},
			CheckFn:          k8s.CheckWorkloads,
			CheckDataBuilder: checkDataBuilder,
			CustomPrinter:    k8s.CheckWorkloadResultPrinter,
		},
	}

	// Run all checks
	ctx := context.Background()
	executor := k8s.NewCheckExecutor(checks)
	logrus.SetLevel(logrus.FatalLevel)

	p := tea.NewProgram(k8s.NewCheckModel(ctx, executor))
	tmp := os.Stdout
	defer func() { os.Stdout = tmp }()
	os.Stdout = nil
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running checks: %v\n", err)
		os.Exit(1)
	}
}

// We should check the following:
// - Phase 1: Environment
//   - Check if /proc/sys/net/ipv4/ip_forward is set to 1
//   - Check if /proc/sys/net/bridge/bridge-nf-call-iptables is set to 1
//   - Check if the machine ID is set
//   - Check if the IP address we are targeting is bound to an interface
//   - Check if the domain name is set
//   - Check if the kubelet service is not runnable
//   - Check if the iknite service is set to run in default mode
//   - Check if /etc/crictl.yaml exists
// - Phase 2: Kubernetes configuration
//   - Check if /etc/kubernetes/pki contains the certificates
//   - Check if /etc/kubernetes/manifests contains the manifests
//   - Check if /etc/kubernetes/{kubelet,scheduler,controller-manager}.conf exists
//   - Check if /etc/kubernetes/{admin,super-admin}.conf exists
//   - Check if /var/lib/etcd contains the etcd data (not empty)
//   - Check if /var/lib/kubelet/config.yaml exists
//   - Check if /var/lib/kubelet/kubeadm-flags.env exists
// - Phase 3: Runtime status
//   - Check if the iknite service is running
//   - Check if the containerd process is running
//   - Check if the kubelet process is running
//   - Check if the kubelet api endpoint (socket) is reachable and healthy
//   - Check if the etcd, kube-apiserver, kube-controller-manager, kube-scheduler pods are running
//   - Check if etcd is healthy
//   - Check if the kube-apiserver is healthy
//  - Phase 4: Workload status
//    - Check if all workloads are ready
/*
		   We want the checks to be run in parallel, so we will use goroutines to run them concurrently. Some checks may depend on the output of other checks, so we will need to wait for the dependent checks to complete before running the dependent checks.
		   We want the status of each task to be displayed in the terminal while running the checks.
		   The status needs to be displayed to the left of the check name. While running the checks, we will display a spinner to indicate that the checks are running.
		   If a check fails, we will display an error message to the right of the check name and the spinner will be replaced with a red cross.
		   If a check passes, we will display a success message to the right of the check name and the spinner will be replaced with a green tick.
		   If a check is skipped, we will display a message to the right of the check name and the spinner will be replaced with a yellow exclamation mark.
	       If a check is waiting for another check to complete, we will display a message to the right of the check name and the spinner will be replaced with a blue ellipsis.
*/
