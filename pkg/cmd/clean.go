package cmd

// cSpell:words txeh
// cSpell: disable
import (
	"os"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/k8s/phases/reset"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	kubeadmOptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
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
	cleanEtcd          bool
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
		cleanEtcd:          false,
		cleanClusterConfig: false,
		cleanIpAddress:     false,
	}
}

func (o *cleanOptions) hasActualWorkToDo() bool {
	return o.stopContainers || o.stopContainerd || o.unmountPaths || o.cleanCni || o.cleanIptables || o.cleanEtcd || o.cleanIpAddress || o.cleanClusterConfig || o.cleanAll
}

func (o *cleanOptions) validate() error {
	if o.cleanAll {
		o.stopContainers = true
		o.stopContainerd = true
		o.unmountPaths = true
		o.cleanCni = true
		o.cleanIptables = true
		o.cleanEtcd = true
		o.cleanIpAddress = true
		o.cleanClusterConfig = true
	}

	return nil
}

func NewCmdClean(ikniteConfig *v1alpha1.IkniteClusterSpec, cleanOptions *cleanOptions) *cobra.Command {
	if cleanOptions == nil {
		cleanOptions = newCleanOptions()
	}

	var cleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "Clean up the environment, stopping all services and removing configuration files (optional)",
		Long: `Kill the cluster and clean up the environment.

This command stops all the services and removes the configuration files. It also
removes the network interfaces and the IP address assigned to the cluster.

This command must be run as root.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cleanOptions.validate(); err != nil {
				log.WithError(err).Error("Invalid options")
				os.Exit(1)
			}

			performClean(ikniteConfig, cleanOptions)
		},
	}

	initializeClean(cleanCmd.Flags(), cleanOptions)
	config.ConfigureClusterCommand(cleanCmd.Flags(), ikniteConfig)

	return cleanCmd
}

func initializeClean(flags *flag.FlagSet, cleanOptions *cleanOptions) {
	flags.BoolVar(&cleanOptions.stopContainers, options.StopContainers, cleanOptions.stopContainers, "Stop containers")
	flags.BoolVar(&cleanOptions.unmountPaths, options.UnmountPaths, cleanOptions.unmountPaths, "Unmount paths")
	flags.BoolVar(&cleanOptions.cleanCni, options.CleanCNI, cleanOptions.cleanCni, "Reset CNI")
	flags.BoolVar(&cleanOptions.cleanIptables, options.CleanIPTables, cleanOptions.cleanIptables, "Reset iptables")
	flags.BoolVar(&cleanOptions.cleanEtcd, options.CleanEtcd, cleanOptions.cleanEtcd, "Reset etcd data")
	flags.BoolVar(&cleanOptions.cleanIpAddress, options.CleanIPAddress, cleanOptions.cleanIpAddress, "Reset IP address")
	flags.BoolVar(&cleanOptions.dryRun, kubeadmOptions.DryRun, cleanOptions.dryRun, "Dry run")
	flags.BoolVar(&cleanOptions.cleanAll, options.CleanAll, cleanOptions.cleanAll, "Reset all")
	flags.BoolVar(&cleanOptions.cleanClusterConfig, options.CleanClusterConfig, cleanOptions.cleanClusterConfig, "Reset cluster configuration")
}

func performClean(ikniteConfig *v1alpha1.IkniteClusterSpec, cleanOptions *cleanOptions) {

	dryRun := cleanOptions.dryRun
	logger := log.WithField("isDryRun", dryRun)

	ikniteCluster, err := v1alpha1.LoadIkniteClusterOrDefault()
	cobra.CheckErr(err)
	state := ikniteCluster.Status.State

	if !state.Stable() {
		log.WithField("State", state).Error("Cluster is not stable")
		os.Exit(1)
	}

	if state == iknite.Running && cleanOptions.hasActualWorkToDo() {
		logger.WithField("serviceName", constants.IkniteService).Info("Stopping iknite service...")
		if !dryRun {
			// TODO: if reset kubelet, remove his note from etcd cluster
			cobra.CheckErr(alpine.StopService(constants.IkniteService))
		}
	}

	// we assume that starting from here, we are in a stopped state
	if cleanOptions.stopContainers {
		logger.Info("Stopping all containers...")
		if err := k8s.StopAllContainers(dryRun); err != nil {
			log.WithError(err).Warn("Error stopping all containers")
		}
	}

	if cleanOptions.stopContainerd {
		logger.WithField("serviceName", constants.ContainerServiceName).Info("Stopping container service...")
		if !dryRun {
			cobra.CheckErr(alpine.StopService(constants.ContainerServiceName))
		}
	}

	if cleanOptions.unmountPaths {
		logger.Info("Unmounting paths...")
		cobra.CheckErr(k8s.UnmountPaths(true, dryRun))
		logger.Info("Removing kubelet runtime files...")
		cobra.CheckErr(k8s.RemoveKubeletFiles(dryRun))
	}

	if cleanOptions.cleanCni {
		cobra.CheckErr(k8s.DeleteCniNamespaces(dryRun))
		cobra.CheckErr(k8s.DeleteNetworkInterfaces(dryRun))
	}

	if cleanOptions.cleanIptables {
		cobra.CheckErr(k8s.ResetIPTables(dryRun))
	}

	if cleanOptions.cleanIpAddress {
		err := k8s.ResetIPAddress(ikniteConfig, dryRun)
		if err != nil {
			log.WithError(err).Warn("Error resetting IP address")
		}
	}

	if cleanOptions.cleanEtcd {
		logger.Info("Cleaning up etcd data...")
		cobra.CheckErr(k8s.DeleteEtcdData(dryRun))
	}

	if cleanOptions.cleanClusterConfig {
		logger.Info("Resetting cluster configuration...")
		reset.CleanConfig(dryRun)
		logger.WithField("path", constants.KubernetesRootConfig).Info("Removing kubernetes root config...")
		if !dryRun {
			os.RemoveAll(constants.KubernetesRootConfig)
		}
	}

	logger.Info("Cleanup completed")

}
