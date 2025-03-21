package cmd

// cSpell:words txeh
// cSpell: disable
import (
	"os"

	s "github.com/bitfield/script"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	kubeadmOptions "k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
)

// cSpell: enable

var (
	dryRun         = false
	stopServices   = true
	stopContainers = true
	unmountPaths   = true
	resetCni       = true
	resetIptables  = true
	resetKubelet   = false
	resetIpAddress = false
)

func NewCmdKillall(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {

	var killallCmd = &cobra.Command{
		Use:   "killall",
		Short: "Kill the cluster and clean up the environment",
		Long: `Kill the cluster and clean up the environment.

This command stops all the services and removes the configuration files. It also
removes the network interfaces and the IP address assigned to the cluster.

This command must be run as root.

`,
		Run: func(cmd *cobra.Command, args []string) {
			performKillall(ikniteConfig)
		},
	}

	initializeKillall(killallCmd.Flags())
	config.ConfigureClusterCommand(killallCmd.Flags(), ikniteConfig)

	return killallCmd
}

func initializeKillall(flags *flag.FlagSet) {
	flags.BoolVar(&stopServices, options.StopServices, stopServices, "Stop the services")
	flags.BoolVar(&stopContainers, options.StopContainers, stopContainers, "Stop containers")
	flags.BoolVar(&unmountPaths, options.UnmountPaths, unmountPaths, "Unmount paths")
	flags.BoolVar(&resetCni, options.ResetCNI, resetCni, "Reset CNI")
	flags.BoolVar(&resetIptables, options.ResetIPTables, resetIptables, "Reset iptables")
	flags.BoolVar(&resetKubelet, options.ResetKubelet, resetKubelet, "Reset kubelet")
	flags.BoolVar(&resetIpAddress, options.ResetIPAddress, resetIpAddress, "Reset IP address")
	flags.BoolVar(&dryRun, kubeadmOptions.DryRun, dryRun, "Dry run")
}

func performKillall(ikniteConfig *v1alpha1.IkniteClusterSpec) {

	if resetKubelet {
		log.Info("Resetting kubelet...")
		_, err := s.Exec("/usr/bin/kubeadm reset --force").Stdout()
		cobra.CheckErr(err)
		os.Remove(constants.KubernetesRootConfig)
	}

	if stopServices {
		log.WithField("DryRun", dryRun).Infof("Stopping %s...", constants.IkniteService)
		if !dryRun {
			cobra.CheckErr(alpine.StopService(constants.IkniteService))
		}

		if stopContainers {
			if err := k8s.StopAllContainers(dryRun); err != nil {
				log.WithError(err).Warn("Error stopping all containers")
			}
		}

		log.WithField("DryRun", dryRun).Infof("Stopping %s...", constants.ContainerServiceName)
		if !dryRun {
			cobra.CheckErr(alpine.StopService(constants.ContainerServiceName))
		}
	}

	if unmountPaths {
		cobra.CheckErr(k8s.UnmountPaths(true, dryRun))
	}

	if stopServices {
		cobra.CheckErr(k8s.RemoveKubeletFiles(dryRun))
	}

	if resetCni {
		cobra.CheckErr(k8s.DeleteCniNamespaces(dryRun))
		cobra.CheckErr(k8s.DeleteNetworkInterfaces(dryRun))
	}

	if resetIptables {
		cobra.CheckErr(k8s.ResetIPTables(dryRun))
	}

	if resetIpAddress {
		cobra.CheckErr(k8s.ResetIPAddress(ikniteConfig, dryRun))
	}
}
