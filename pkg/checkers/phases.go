// cSpell: words runlevels
package checkers

import (
	"fmt"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewEnvironmentCheckPhase(ikniteConfig *v1alpha1.IkniteClusterSpec) *check.Check {
	return check.NewPhase("environment", "Environment configuration",
		FileCheck(
			"ip_forward",
			"Check IP forwarding is enabled",
			"/proc/sys/net/ipv4/ip_forward",
			"1\n",
		),
		FileCheck("bridge_nf_call_iptables", "Check IP Tables is active for bridges",
			"/proc/sys/net/bridge/bridge-nf-call-iptables", "1\n"),
		FileCheck("machine_id", "Check machine id is defined", "/etc/machine-id", ""),
		FileCheck(
			"crictl_yaml",
			"Check crictl configuration is defined",
			"/etc/crictl.yaml",
			"",
		),
		//   - Check if the kubelet service is not runnable
		NewPreventedServiceCheck("kubelet"),
		//   - Check if the iknite service is set to run in default mode
		FileCheck("iknite_service", "Check if iknite is active on default runlevel",
			"/etc/runlevels/default/iknite", ""),
		//   - Check if the IP address we are targeting is bound to an interface
		NewIpBoundCheck(),
		//   - Check if the domain name is set
		DomainNameCheck(ikniteConfig.DomainName, ikniteConfig.Ip),
	)
}

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

func NewConfigurationCheckPhase(ikniteConfig *v1alpha1.IkniteClusterSpec) *check.Check {
	var apiBackendName string
	if ikniteConfig.UseEtcd {
		apiBackendName = constants.EtcdBackendName
	} else {
		apiBackendName = constants.KineBackendName
	}
	dBManifestCheck := SimpleFileCheck(
		fmt.Sprintf("manifest_%s", apiBackendName),
		fmt.Sprintf("/etc/kubernetes/manifests/%s.yaml", apiBackendName),
	)

	return check.NewPhase("configuration", "Kubernetes configuration",
		FileTreeCheck("pki", "Check PKI files", "/etc/kubernetes/pki", pkiFiles),
		check.NewPhase("manifests", "Kubernetes manifests",
			dBManifestCheck,
			SimpleFileCheck(
				"manifest_apiserver",
				"/etc/kubernetes/manifests/kube-apiserver.yaml",
			),
			SimpleFileCheck(
				"manifest_controller",
				"/etc/kubernetes/manifests/kube-controller-manager.yaml",
			),
			SimpleFileCheck(
				"manifest_scheduler",
				"/etc/kubernetes/manifests/kube-scheduler.yaml",
			),
		),
		SimpleFileCheck("kubelet_conf", "/etc/kubernetes/kubelet.conf"),
		SimpleFileCheck("admin_conf", "/etc/kubernetes/admin.conf"),
		SimpleFileCheck("kubelet_config", "/var/lib/kubelet/config.yaml"),
		SimpleFileCheck("kubeadm_flags", "/var/lib/kubelet/kubeadm-flags.env"),
		SimpleFileCheck("iknite_conf", "/etc/kubernetes/iknite.conf"),
		// Check that the etcd data directory is present and contains data
		APIBackendDataCheck(apiBackendName),
	)
}

func NewRuntimeCheckPhase(waitOptions *utils.WaitOptions) *check.Check {
	// Phase 3: Runtime status
	return check.NewPhase("runtime", "Runtime status",
		// Check that openrc is started
		OpenRCCheck(),
		ServiceCheck("iknite_running", "iknite", ServiceTypeOpenRC),
		ServiceCheck("containerd_running", "containerd", ServiceTypeOpenRC),
		ServiceCheck("buildkitd_running", "buildkitd", ServiceTypeOpenRC),
		//  - Check if the kubelet process is running
		ServiceCheck("kubelet_running", "kubelet", ServiceTypePidFile, "iknite_running"),
		//   - Check if the kubelet api endpoint (socket) is reachable and healthy
		NewKubeletHealthCheck(waitOptions.CheckTimeout),
		//   - Check if the kube-apiserver is healthy
		NewApiServerHealthCheck(waitOptions.CheckTimeout),
		//   - Check if the iknite status server is healthy
		NewIkniteServerHealthCheck(),
	)
}

func NewWorkloadStatusCheck() *check.Check {
	return &check.Check{
		Name:          "workload_status",
		Description:   "Check Workload Status",
		DependsOn:     []string{"runtime"},
		CheckFn:       CheckWorkloads,
		CustomPrinter: CheckWorkloadResultPrinter,
	}
}

func ConfigureIkniteClusterChecker(
	executor *check.CheckExecutor,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	waitOptions *utils.WaitOptions,
) {
	executor.AddCheck(NewEnvironmentCheckPhase(ikniteConfig))
	executor.AddCheck(NewConfigurationCheckPhase(ikniteConfig))
	executor.AddCheck(NewRuntimeCheckPhase(waitOptions))
	executor.AddCheck(NewWorkloadStatusCheck())
}
