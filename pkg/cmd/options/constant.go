package options

const (
	// General.
	Config           = "config"
	Verbosity        = "verbosity"
	Json             = "json"
	Timeout          = "timeout"
	Wait             = "wait"
	Watch            = "watch"
	CheckTimeout     = "check-timeout"
	CheckInterval    = "check-interval"
	CheckRetries     = "check-retries"
	CheckOkResponses = "check-ok-responses"
	CheckImmediate   = "check-immediate"

	// Kustomization.
	Kustomization = "kustomization"
	ForceConfig   = "force-config"
	ForceEmbedded = "force-embedded"

	// Configuration.
	Ip                 = "ip"
	IpCreate           = "create-ip"
	IpNetworkInterface = "network-interface"
	DomainName         = "domain-name"
	EnableMDNS         = "enable-mdns"
	ClusterName        = "cluster-name"

	// Etcd/Kine.
	UseEtcd = "use-etcd"

	// Clean.
	StopContainers     = "stop-containers"
	UnmountPaths       = "unmount-paths"
	CleanCNI           = "clean-cni"
	CleanIPTables      = "clean-iptables"
	CleanAPIBackend    = "clean-api-backend"
	CleanIPAddress     = "clean-ip-address"
	CleanAll           = "clean-all"
	CleanClusterConfig = "clean-cluster-config"

	// Info.
	OutputFormat      = "output-format"
	OutputDestination = "output-destination"
)
