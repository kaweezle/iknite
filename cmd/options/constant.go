package options

const (
	// General
	Config    = "config"
	Verbosity = "verbosity"
	Json      = "json"

	// Kustomization
	KustomizeDirectory      = "kustomize-directory"
	Wait                    = "wait"
	ForceConfig             = "force-config"
	ClusterCheckWait        = "cluster-check-wait"
	ClusterCheckRetries     = "cluster-check-retries"
	ClusterCheckOkResponses = "cluster-check-ok-responses"

	// Configuration
	Ip                 = "ip"
	IpCreate           = "ip-create"
	IpNetworkInterface = "ip-network-interface"
	DomainName         = "domain-name"
	EnableMDNS         = "enable-mdns"
	ClusterName        = "cluster-name"

	// Killall
	StopServices   = "stop-services"
	StopContainers = "stop-containers"
	UnmountPaths   = "unmount-paths"
	ResetCNI       = "reset-cni"
	ResetIPTables  = "reset-iptables"
	ResetKubelet   = "reset-kubelet"
	ResetIPAddress = "reset-ip-address"
)
