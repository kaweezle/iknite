package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewKubeletCmd(ikniteConfig *v1alpha1.IkniteClusterSpec, kustomizeOptions *utils.KustomizeOptions) *cobra.Command {
	if kustomizeOptions == nil {
		kustomizeOptions = utils.NewKustomizeOptions()
	}
	kubeletCmd := &cobra.Command{
		Use:   "kubelet",
		Short: "Start and monitor the kubelet (Experimental)",
		Long: `Starts and monitors the kubelet.

The kubelet is started and monitored. The following operations are performed:
- Starts the kubelet,
- Monitors the kubelet,
`,
		Run: func(_ *cobra.Command, _ []string) { performKubelet(ikniteConfig, kustomizeOptions) },
	}

	config.AddIkniteClusterFlags(kubeletCmd.Flags(), ikniteConfig)
	utils.AddKustomizeOptionsFlags(kubeletCmd.Flags(), kustomizeOptions)

	return kubeletCmd
}

func performKubelet(ikniteConfig *v1alpha1.IkniteClusterSpec, kustomizeOptions *utils.KustomizeOptions) {
	alpineHost := alpine.NewDefaultAlpineHost()
	cobra.CheckErr(k8s.PrepareKubernetesEnvironment(alpineHost, ikniteConfig))
	cobra.CheckErr(k8s.StartAndConfigureKubelet(ikniteConfig, kustomizeOptions))
}
