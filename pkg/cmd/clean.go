package cmd

// cSpell: words txeh ikniteapi
import (
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	kubeadmOptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/cmd/util"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/k8s/phases/reset"
	"github.com/kaweezle/iknite/pkg/utils"
)

type cleanOptions struct {
	dryRun             bool
	cleanAll           bool
	stopContainers     bool
	stopContainerd     bool
	unmountPaths       bool
	cleanCni           bool
	cleanIptables      bool
	cleanAPIBackend    bool
	cleanClusterConfig bool
	cleanIpAddress     bool
}

func newCleanOptions() *cleanOptions {
	return &cleanOptions{
		dryRun:             false,
		cleanAll:           false,
		stopContainers:     true,
		stopContainerd:     false,
		unmountPaths:       true,
		cleanCni:           true,
		cleanIptables:      true,
		cleanAPIBackend:    false,
		cleanClusterConfig: false,
		cleanIpAddress:     false,
	}
}

func (o *cleanOptions) hasActualWorkToDo() bool {
	return o.stopContainers || o.stopContainerd || o.unmountPaths || o.cleanCni ||
		o.cleanIptables || o.cleanAPIBackend || o.cleanIpAddress || o.cleanClusterConfig ||
		o.cleanAll
}

//nolint:unparam // validate may be extended in the future
func (o *cleanOptions) validate() error {
	if o.cleanAll {
		o.stopContainers = true
		o.stopContainerd = true
		o.unmountPaths = true
		o.cleanCni = true
		o.cleanIptables = true
		o.cleanAPIBackend = true
		o.cleanIpAddress = true
		o.cleanClusterConfig = true
	}

	return nil
}

func NewCmdClean(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	cleanOptions *cleanOptions,
	alpineHost host.Host,
) *cobra.Command {
	if cleanOptions == nil {
		cleanOptions = newCleanOptions()
	}
	if alpineHost == nil {
		alpineHost = host.NewDefaultHost()
	}

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up the environment, stopping all services and removing configuration files (optional)",
		Long: `Kill the cluster and clean up the environment.

This command stops all the services and removes the configuration files. It also
removes the network interfaces and the IP address assigned to the cluster.

This command must be run as root.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := util.LoggerFromCommand(cmd)
			if err := cleanOptions.validate(); err != nil {
				logger.Error("Invalid options", utils.ErrorKey, err)
				return fmt.Errorf("invalid options: %w", err)
			}

			return performClean(alpineHost, ikniteConfig, cleanOptions, logger)
		},
	}

	initializeClean(cleanCmd.Flags(), cleanOptions)
	config.AddIkniteClusterFlags(cleanCmd.Flags(), ikniteConfig)

	return cleanCmd
}

func initializeClean(flags *flag.FlagSet, cleanOptions *cleanOptions) {
	flags.BoolVar(
		&cleanOptions.stopContainers,
		options.StopContainers,
		cleanOptions.stopContainers,
		"Stop containers",
	)
	flags.BoolVar(
		&cleanOptions.unmountPaths,
		options.UnmountPaths,
		cleanOptions.unmountPaths,
		"Unmount paths",
	)
	flags.BoolVar(&cleanOptions.cleanCni, options.CleanCNI, cleanOptions.cleanCni, "Reset CNI")
	flags.BoolVar(
		&cleanOptions.cleanIptables,
		options.CleanIPTables,
		cleanOptions.cleanIptables,
		"Reset iptables",
	)
	flags.BoolVar(
		&cleanOptions.cleanAPIBackend,
		options.CleanAPIBackend,
		cleanOptions.cleanAPIBackend,
		"Reset API backend data",
	)
	flags.BoolVar(
		&cleanOptions.cleanIpAddress,
		options.CleanIPAddress,
		cleanOptions.cleanIpAddress,
		"Reset IP address",
	)
	flags.BoolVar(&cleanOptions.dryRun, kubeadmOptions.DryRun, cleanOptions.dryRun, "Dry run")
	flags.BoolVar(&cleanOptions.cleanAll, options.CleanAll, cleanOptions.cleanAll, "Reset all")
	flags.BoolVar(&cleanOptions.cleanClusterConfig, options.CleanClusterConfig,
		cleanOptions.cleanClusterConfig, "Reset cluster configuration")
}

//nolint:gocyclo,gocognit // TODO: Should use a runner pattern to reduce complexity
func performClean(
	alpineHost host.Host,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	cleanOptions *cleanOptions,
	l *slog.Logger,
) error {
	dryRun := cleanOptions.dryRun
	cleaner := k8s.NewCleaner(alpineHost, l, ikniteConfig, dryRun)

	if !cleanOptions.hasActualWorkToDo() {
		cleaner.Info("No cleanup actions specified, skipping cleanup")
		return nil
	}

	state := iknite.Undefined
	ikniteCluster, err := v1alpha1.LoadIkniteCluster(alpineHost)
	if err != nil {
		if !os.IsNotExist(err) {
			cleaner.Warn("Failed to load iknite cluster, assuming it does not exist", utils.ErrorKey, err)
		}
	} else {
		cleaner.Info("Loaded iknite cluster status. Replace cluster config with the one from the status file")
		state = ikniteCluster.Status.State
		*ikniteConfig = ikniteCluster.Spec
	}

	if !state.Stable() {
		return fmt.Errorf("cluster is not in a stable state: %s", state)
	}

	if state == iknite.Running {
		cleaner.Info("Stopping iknite service...", "serviceName", constants.IkniteService)
		if !dryRun {
			// TODO: if reset kubelet, remove his node from etcd cluster
			err = alpine.StopService(alpineHost, constants.IkniteService, cleaner.Logger)
			if err != nil {
				return fmt.Errorf("failed to stop iknite service: %w", err)
			}
		}
	}

	// we assume that starting from here, we are in a stopped state
	if cleanOptions.stopContainers {
		cleaner.Info("Stopping all containers...")
		if err = cleaner.StopAllContainers(); err != nil {
			cleaner.Warn("Error stopping all containers", utils.ErrorKey, err)
		}
	}

	if cleanOptions.stopContainerd {
		cleaner.Info("Stopping container service...", "serviceName", constants.ContainerServiceName)
		if !dryRun {
			err = alpine.StopService(alpineHost, constants.ContainerServiceName, cleaner.Logger)
			if err != nil {
				return fmt.Errorf("failed to stop container service: %w", err)
			}
		}
	}

	if cleanOptions.unmountPaths {
		cleaner.Info("Unmounting paths...")
		err = cleaner.UnmountPaths(true)
		if err != nil {
			return fmt.Errorf("failed to unmount paths: %w", err)
		}
		cleaner.Info("Removing kubelet runtime files...")
		err = cleaner.RemoveKubeletFiles()
		if err != nil {
			return fmt.Errorf("failed to remove kubelet runtime files: %w", err)
		}
	}

	if cleanOptions.cleanCni {
		err = cleaner.DeleteCniNamespaces()
		if err != nil {
			return fmt.Errorf("failed to delete CNI namespaces: %w", err)
		}
		err = cleaner.DeleteNetworkInterfaces()
		if err != nil {
			return fmt.Errorf("failed to delete network interfaces: %w", err)
		}
	}

	if cleanOptions.cleanIptables {
		err = cleaner.ResetIPTables()
		if err != nil {
			return fmt.Errorf("failed to reset iptables: %w", err)
		}
	}

	if cleanOptions.cleanIpAddress {
		err = cleaner.ResetIPAddress()
		if err != nil {
			cleaner.Warn("Error resetting IP address", utils.ErrorKey, err)
		}
	}

	if cleanOptions.cleanAPIBackend {
		cleaner.Info("Cleaning up API backend data...")
		apiBackendName := "kine"
		if ikniteConfig.UseEtcd {
			apiBackendName = constants.EtcdBackendName
		}
		err = cleaner.DeleteAPIBackendData(apiBackendName, ikniteConfig.APIBackendDatabaseDirectory)
		if err != nil {
			return fmt.Errorf("failed to delete API backend data: %w", err)
		}
	}

	if cleanOptions.cleanClusterConfig {
		cleaner.Info("Resetting cluster configuration...")
		reset.CleanConfig(alpineHost, dryRun, cleaner.Logger)
		cleaner.Info("Removing kubernetes root config...", "path", constants.KubernetesRootConfig)
		if !dryRun {
			err = alpineHost.RemoveAll(constants.KubernetesRootConfig)
			if err != nil {
				return fmt.Errorf("failed to remove kubernetes root config: %w", err)
			}
		}
	}

	_, kubeletProcess, err := alpine.CheckPidFile(alpineHost, "kubelet", cleaner.Logger)
	switch {
	case err != nil:
		cleaner.Warn("Error checking kubelet process", utils.ErrorKey, err)
	case kubeletProcess != nil:
		cleaner.Info("Kubelet is still running, stopping it...", "pid", kubeletProcess.Pid())
		if !dryRun {
			err = kubeletProcess.Signal(syscall.SIGTERM)
			if err == nil {
				cleaner.Info("Waiting for kubelet to stop...", "pid", kubeletProcess.Pid())
				err = kubeletProcess.Wait()
				if err != nil {
					return fmt.Errorf("failed to wait for kubelet process to stop: %w", err)
				}
			}
		}
	default:
		cleaner.Info("Kubelet is not running")
	}

	cleaner.Info("Cleanup completed")
	return nil
}
