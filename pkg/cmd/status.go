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
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/checkers"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
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
	"iknite-client.crt",
	"iknite-client.key",
	"iknite-server.crt",
	"iknite-server.key",
}

func NewStatusCmd(ikniteConfig *v1alpha1.IkniteClusterSpec, waitOptions *utils.WaitOptions) *cobra.Command {
	if waitOptions == nil {
		waitOptions = utils.NewWaitOptions()
		waitOptions.OkResponses = 3
	}
	// configureCmd represents the start command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Gives status information on the cluster",
		Long: `Gives status information of the deployed workloads:

- Deployments
- Daemonsets
- Statefulsets
`,
		Run: func(_ *cobra.Command, _ []string) {
			performStatus(host.NewDefaultHost(), ikniteConfig, waitOptions)
		},
	}

	flags := statusCmd.Flags()
	config.AddIkniteClusterFlags(flags, ikniteConfig)
	utils.AddWaitOptionsFlags(flags, waitOptions)

	return statusCmd
}

func performStatus(alpineHost host.Host, ikniteConfig *v1alpha1.IkniteClusterSpec, waitOptions *utils.WaitOptions) {
	checkData := checkers.CreateCheckWorkloadData(ikniteConfig.GetApiEndPoint(), waitOptions, alpineHost)

	var apiBackendName string
	if ikniteConfig.UseEtcd {
		apiBackendName = constants.EtcdBackendName
	} else {
		apiBackendName = constants.KineBackendName
	}
	dBManifestCheck := checkers.SimpleFileCheck(
		fmt.Sprintf("manifest_%s", apiBackendName),
		fmt.Sprintf("/etc/kubernetes/manifests/%s.yaml", apiBackendName),
	)

	// Create all checks
	checks := []*check.Check{
		// Phase 1: Environment
		check.NewPhase("environment", "Environment configuration",
			checkers.FileCheck(
				"ip_forward",
				"Check IP forwarding is enabled",
				"/proc/sys/net/ipv4/ip_forward",
				"1\n",
			),
			checkers.FileCheck("bridge_nf_call_iptables", "Check IP Tables is active for bridges",
				"/proc/sys/net/bridge/bridge-nf-call-iptables", "1\n"),
			checkers.FileCheck("machine_id", "Check machine id is defined", "/etc/machine-id", ""),
			checkers.FileCheck(
				"crictl_yaml",
				"Check crictl configuration is defined",
				"/etc/crictl.yaml",
				"",
			),
			//   - Check if the kubelet service is not runnable
			&check.Check{
				Name:        "kubelet_service",
				Description: "Check if the kubelet service is not runnable",
				CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
					data, ok := checkData.(checkers.CheckWorkloadData)
					if !ok {
						return false, "", fmt.Errorf("invalid check data type")
					}
					runnable, err := k8s.IsKubeletServiceRunnable(data.Host(), constants.RcConfFile)
					if err != nil {
						return false, "", fmt.Errorf(
							"failed to check if kubelet service is runnable: %w",
							err,
						)
					}
					if runnable {
						return false, "Kubelet service is runnable", nil
					}
					return true, "/etc/rc.conf hack preventing kubelet from running in place", nil
				},
			},
			//   - Check if the iknite service is set to run in default mode
			checkers.FileCheck("iknite_service", "Check if iknite is active on default runlevel",
				"/etc/runlevels/default/iknite", ""),
			//   - Check if the IP address we are targeting is bound to an interface
			&check.Check{
				Name:        "ip_bound",
				Description: "Check if the IP address is bound to an interface",
				CheckFn: func(_ context.Context, checkData check.CheckData) (bool, string, error) {
					data, ok := checkData.(checkers.CheckWorkloadData)
					if !ok {
						return false, "", fmt.Errorf("invalid check data type")
					}
					if ikniteConfig.CreateIp {
						result, err := data.Host().CheckIpExists(ikniteConfig.Ip)
						switch {
						case err != nil:
							return false, "", fmt.Errorf("failed to check if IP exists: %w", err)
						case result:
							return true, fmt.Sprintf(
								"IP address %s is bound to an interface",
								ikniteConfig.Ip.String(),
							), nil
						default:
							return false, fmt.Sprintf(
								"IP address %s is not bound to any interface",
								ikniteConfig.Ip.String(),
							), nil
						}
					} else {
						return true, "Don't need to create IP", nil
					}
				},
			},
			//   - Check if the domain name is set
			checkers.DomainNameCheck(ikniteConfig.DomainName, ikniteConfig.Ip),
		),

		// Phase 2: Kubernetes configuration
		check.NewPhase("configuration", "Kubernetes configuration",
			checkers.FileTreeCheck("pki", "Check PKI files", "/etc/kubernetes/pki", pkiFiles),
			check.NewPhase("manifests", "Kubernetes manifests",
				dBManifestCheck,
				checkers.SimpleFileCheck(
					"manifest_apiserver",
					"/etc/kubernetes/manifests/kube-apiserver.yaml",
				),
				checkers.SimpleFileCheck(
					"manifest_controller",
					"/etc/kubernetes/manifests/kube-controller-manager.yaml",
				),
				checkers.SimpleFileCheck(
					"manifest_scheduler",
					"/etc/kubernetes/manifests/kube-scheduler.yaml",
				),
			),
			checkers.SimpleFileCheck("kubelet_conf", "/etc/kubernetes/kubelet.conf"),
			checkers.SimpleFileCheck("admin_conf", "/etc/kubernetes/admin.conf"),
			checkers.SimpleFileCheck("kubelet_config", "/var/lib/kubelet/config.yaml"),
			checkers.SimpleFileCheck("kubeadm_flags", "/var/lib/kubelet/kubeadm-flags.env"),
			checkers.SimpleFileCheck("iknite_conf", "/etc/kubernetes/iknite.conf"),
			// Check that the etcd data directory is present and contains data
			checkers.APIBackendDataCheck(apiBackendName),
		),

		// Phase 3: Runtime status
		check.NewPhase("runtime", "Runtime status",
			// Check that openrc is started
			checkers.OpenRCCheck(),
			checkers.ServiceCheck("iknite_running", "iknite", checkers.ServiceTypeOpenRC),
			checkers.ServiceCheck("containerd_running", "containerd", checkers.ServiceTypeOpenRC),
			checkers.ServiceCheck("buildkitd_running", "buildkitd", checkers.ServiceTypeOpenRC),
			//  - Check if the kubelet process is running
			checkers.ServiceCheck("kubelet_running", "kubelet", checkers.ServiceTypePidFile, "iknite_running"),
			//   - Check if the kubelet api endpoint (socket) is reachable and healthy
			&check.Check{
				Name:        "kubelet_health",
				DependsOn:   []string{"kubelet_running"},
				Description: "Check if the kubelet is reachable and healthy",
				CheckFn: func(_ context.Context, _ check.CheckData) (bool, string, error) {
					return checkers.CheckKubeletHealth(waitOptions.CheckTimeout)
				},
			},
			//   - Check if the kube-apiserver is healthy
			&check.Check{
				Name:        "apiserver_health",
				DependsOn:   []string{"kubelet_running"},
				Description: "Check if the kube-apiserver is healthy",
				CheckFn: func(_ context.Context, data check.CheckData) (bool, string, error) {
					return checkers.CheckApiServerHealth(waitOptions.CheckTimeout, data)
				},
			},
			//   - Check if the iknite status server is healthy
			&check.Check{
				Name:        "iknite_server_health",
				DependsOn:   []string{"apiserver_health"},
				Description: "Check if the iknite status server is healthy",
				CheckFn: func(ctx context.Context, _ check.CheckData) (bool, string, error) {
					waitOptions := utils.NewWaitOptions()
					waitOptions.Retries = 3
					waitOptions.Timeout = 15 * time.Second

					return checkers.CheckIkniteServerHealth(ctx, waitOptions)
				},
			},
		),
		{
			Name:          "workload_status",
			Description:   "Check Workload Status",
			DependsOn:     []string{"runtime"},
			CheckFn:       checkers.CheckWorkloads,
			CustomPrinter: checkers.CheckWorkloadResultPrinter,
		},
	}

	// Run all checks
	ctx := context.Background()
	executor := check.NewCheckExecutor(checks, checkData)
	logrus.SetLevel(logrus.FatalLevel)

	p := tea.NewProgram(check.NewCheckModel(ctx, executor))
	tmp := os.Stdout
	defer func() { os.Stdout = tmp }()
	os.Stdout = nil
	if _, err := p.Run(); err != nil {
		cobra.CheckErr(fmt.Errorf("error running checks: %w", err))
	}
}
