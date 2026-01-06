package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/k8s"
)

func NewKubeletCmd(ikniteConfig *v1alpha1.IkniteClusterSpec) *cobra.Command {
	kubeletCmd := &cobra.Command{
		Use:   "kubelet",
		Short: "Start and monitor the kubelet (Experimental)",
		Long: `Starts and monitors the kubelet.

The kubelet is started and monitored. The following operations are performed:
- Starts the kubelet,
- Monitors the kubelet,
`,
		Run:              func(cmd *cobra.Command, args []string) { performKubelet(ikniteConfig) },
		PersistentPreRun: config.StartPersistentPreRun,
	}

	config.ConfigureClusterCommand(kubeletCmd.Flags(), ikniteConfig)
	initializeKustomization(kubeletCmd.Flags())

	return kubeletCmd
}

func performKubelet(ikniteConfig *v1alpha1.IkniteClusterSpec) {
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(ikniteConfig))
	cobra.CheckErr(k8s.StartAndConfigureKubelet(ikniteConfig))
}
