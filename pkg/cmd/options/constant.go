package options

const (
	// General
	Config    = "config"
	Verbosity = "verbosity"
	Json      = "json"
	Timeout   = "timeout"

	// Kustomization
	Kustomization           = "kustomization"
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

	// Clean
	StopContainers     = "stop-containers"
	UnmountPaths       = "unmount-paths"
	CleanCNI           = "clean-cni"
	CleanIPTables      = "clean-iptables"
	CleanEtcd          = "clean-etcd"
	CleanIPAddress     = "clean-ip-address"
	CleanAll           = "clean-all"
	CleanClusterConfig = "clean-cluster-config"
)
