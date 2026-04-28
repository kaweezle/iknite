package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

func NewKubeletCmd(
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	kustomizeOptions *utils.KustomizeOptions,
	alpineHost host.Host,
) *cobra.Command {
	if kustomizeOptions == nil {
		kustomizeOptions = utils.NewKustomizeOptions()
	}
	if alpineHost == nil {
		alpineHost = host.NewDefaultHost()
	}
	kubeletCmd := &cobra.Command{
		Use:   "kubelet",
		Short: "Start and monitor the kubelet (Experimental)",
		Long: `Starts and monitors the kubelet.

The kubelet is started and monitored. The following operations are performed:
- Starts the kubelet,
- Monitors the kubelet,
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return performKubelet(cmd.Context(), ikniteConfig, kustomizeOptions, alpineHost)
		},
	}

	config.AddIkniteClusterFlags(kubeletCmd.Flags(), ikniteConfig)
	utils.AddKustomizeOptionsFlags(kubeletCmd.Flags(), kustomizeOptions)

	return kubeletCmd
}

func performKubelet(
	ctx context.Context,
	ikniteConfig *v1alpha1.IkniteClusterSpec,
	kustomizeOptions *utils.KustomizeOptions,
	alpineHost host.Host,
) error {
	kubeClient, err := k8s.NewDefaultClient(alpineHost)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	err = k8s.PrepareKubernetesEnvironment(ctx, alpineHost, ikniteConfig)
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes environment: %w", err)
	}

	runtime := k8s.NewKubeletRuntime(alpineHost, kubeClient)
	err = k8s.StartAndConfigureKubelet(ctx, runtime, kustomizeOptions)
	if err != nil {
		return fmt.Errorf("failed to start and configure kubelet: %w", err)
	}
	return nil
}
