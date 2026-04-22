package cmd

// cSpell:words txeh
// cSpell: disable
import (
	"fmt"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	kubeadmOptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/k8s/phases/reset"
)

// cSpell: enable

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
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := cleanOptions.validate(); err != nil {
				log.WithError(err).Error("Invalid options")
				return fmt.Errorf("invalid options: %w", err)
			}

			return performClean(alpineHost, ikniteConfig, cleanOptions)
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
func performClean(alpineHost host.Host, ikniteConfig *v1alpha1.IkniteClusterSpec, cleanOptions *cleanOptions) error {
	dryRun := cleanOptions.dryRun
	logger := log.WithField("isDryRun", dryRun)

	if !cleanOptions.hasActualWorkToDo() {
		logger.Info("No cleanup actions specified, skipping cleanup")
		return nil
	}

	ikniteCluster, err := v1alpha1.LoadIkniteClusterOrDefault(alpineHost)
	if err != nil {
		return fmt.Errorf("failed to load iknite cluster: %w", err)
	}
	state := ikniteCluster.Status.State

	if !state.Stable() {
		return fmt.Errorf("cluster is not in a stable state: %s", state)
	}

	if state == iknite.Running {
		logger.WithField("serviceName", constants.IkniteService).Info("Stopping iknite service...")
		if !dryRun {
			// TODO: if reset kubelet, remove his node from etcd cluster
			err = alpine.StopService(alpineHost, constants.IkniteService)
			if err != nil {
				return fmt.Errorf("failed to stop iknite service: %w", err)
			}
		}
	}

	// we assume that starting from here, we are in a stopped state
	if cleanOptions.stopContainers {
		logger.Info("Stopping all containers...")
		if err = k8s.StopAllContainers(alpineHost, dryRun); err != nil {
			log.WithError(err).Warn("Error stopping all containers")
		}
	}

	if cleanOptions.stopContainerd {
		logger.WithField("serviceName", constants.ContainerServiceName).
			Info("Stopping container service...")
		if !dryRun {
			err = alpine.StopService(alpineHost, constants.ContainerServiceName)
			if err != nil {
				return fmt.Errorf("failed to stop container service: %w", err)
			}
		}
	}

	if cleanOptions.unmountPaths {
		logger.Info("Unmounting paths...")
		err = k8s.UnmountPaths(alpineHost, true, dryRun)
		if err != nil {
			return fmt.Errorf("failed to unmount paths: %w", err)
		}
		logger.Info("Removing kubelet runtime files...")
		err = k8s.RemoveKubeletFiles(alpineHost, dryRun)
		if err != nil {
			return fmt.Errorf("failed to remove kubelet runtime files: %w", err)
		}
	}

	if cleanOptions.cleanCni {
		err = k8s.DeleteCniNamespaces(alpineHost, dryRun)
		if err != nil {
			return fmt.Errorf("failed to delete CNI namespaces: %w", err)
		}
		err = k8s.DeleteNetworkInterfaces(alpineHost, dryRun)
		if err != nil {
			return fmt.Errorf("failed to delete network interfaces: %w", err)
		}
	}

	if cleanOptions.cleanIptables {
		err = k8s.ResetIPTables(alpineHost, dryRun)
		if err != nil {
			return fmt.Errorf("failed to reset iptables: %w", err)
		}
	}

	if cleanOptions.cleanIpAddress {
		err = k8s.ResetIPAddress(alpineHost, ikniteConfig, dryRun)
		if err != nil {
			log.WithError(err).Warn("Error resetting IP address")
		}
	}

	if cleanOptions.cleanAPIBackend {
		logger.Info("Cleaning up API backend data...")
		apiBackendName := "kine"
		if ikniteConfig.UseEtcd {
			apiBackendName = constants.EtcdBackendName
		}
		err = k8s.DeleteAPIBackendData(alpineHost, dryRun, apiBackendName, ikniteConfig.APIBackendDatabaseDirectory)
		if err != nil {
			return fmt.Errorf("failed to delete API backend data: %w", err)
		}
	}

	if cleanOptions.cleanClusterConfig {
		logger.Info("Resetting cluster configuration...")
		reset.CleanConfig(alpineHost, dryRun)
		logger.WithField("path", constants.KubernetesRootConfig).
			Info("Removing kubernetes root config...")
		if !dryRun {
			err = alpineHost.RemoveAll(constants.KubernetesRootConfig)
			if err != nil {
				return fmt.Errorf("failed to remove kubernetes root config: %w", err)
			}
		}
	}

	_, kubeletProcess, err := alpine.CheckPidFile(alpineHost, "kubelet")
	if err != nil {
		logger.WithError(err).Warn("Error checking kubelet process")
	} else if kubeletProcess != nil {
		logger.WithField("pid", kubeletProcess.Pid).Info("Kubelet is still running, stopping it...")
		if !dryRun {
			err = kubeletProcess.Signal(syscall.SIGTERM)
			if err == nil {
				logger.WithField("pid", kubeletProcess.Pid).Info("Waiting for kubelet to stop...")
				err = kubeletProcess.Wait()
				if err != nil {
					return fmt.Errorf("failed to wait for kubelet process to stop: %w", err)
				}
			}
		}
	}

	logger.Info("Cleanup completed")
	return nil
}
