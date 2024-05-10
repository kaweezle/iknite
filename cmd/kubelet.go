package cmd

import (
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/spf13/cobra"
)

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

func init() {
	rootCmd.AddCommand(kubeletCmd)
	configureClusterCommand(kubeletCmd)
	initializeKustomization(kubeletCmd)
}

func performKubelet(cmd *cobra.Command, args []string) {
	clusterConfig, err := PrepareKubernetesEnvironment()
	cobra.CheckErr(err)
	cobra.CheckErr(k8s.StartKubelet(clusterConfig))
}
