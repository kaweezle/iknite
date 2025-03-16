package cmd

import (
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/spf13/cobra"
)

func NewKubletCmd() *cobra.Command {

	var kubeletCmd = &cobra.Command{
		Use:   "kubelet",
		Short: "Start and monitor the kubelet (Experimental)",
		Long: `Starts and monitors the kubelet.

The kubelet is started and monitored. The following operations are performed:
- Starts the kubelet,
- Monitors the kubelet,
`,
		Run:              performKubelet,
		PersistentPreRun: startPersistentPreRun,
	}

	configureClusterCommand(kubeletCmd.Flags(), ikniteConfig)
	initializeKustomization(kubeletCmd.Flags())

	return kubeletCmd
}

func performKubelet(cmd *cobra.Command, args []string) {
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ikniteConfig))
	cobra.CheckErr(k8s.StartAndConfigureKubelet(ikniteConfig))
}
