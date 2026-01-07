/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

// cSpell:words dedent IPVS ipvsadm kubeadmapiv1 kubeadmapi klog
// cSpell: disable
import (
	"errors"
	"fmt"
	"io"
	"os"
	_ "unsafe"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmScheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	phases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/reset"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	cmdUtil "k8s.io/kubernetes/cmd/kubeadm/app/cmd/util"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	apiClient "k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	configUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	kubeconfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	iknitePhases "github.com/kaweezle/iknite/pkg/k8s/phases/reset"
)

// cSpell: enable

//go:linkname AddResetFlags k8s.io/kubernetes/cmd/kubeadm/app/cmd.AddResetFlags
func AddResetFlags(flagSet *flag.FlagSet, resetOptions *resetOptions)

//go:linkname resetDetectCRISocket k8s.io/kubernetes/cmd/kubeadm/app/cmd.resetDetectCRISocket
func resetDetectCRISocket(
	resetCfg *kubeadmapi.ResetConfiguration, initCfg *kubeadmapi.InitConfiguration) (string, error)

// resetOptions defines all the options exposed via flags by kubeadm reset.
//
//nolint:govet // data structure passed to kubeadm phases
type resetOptions struct {
	kubeconfigPath        string
	cfgPath               string
	ignorePreflightErrors []string
	externalCfg           *v1beta4.ResetConfiguration
	skipCRIDetect         bool
	ikniteCfg             *v1alpha1.IkniteClusterSpec
}

// resetData defines all the runtime information used when running the kubeadm reset workflow;
// this data is shared across all the phases that are included in the workflow.
//
//nolint:govet // data structure passed to kubeadm phases
type resetData struct {
	certificatesDir       string
	client                clientset.Interface
	criSocketPath         string
	forceReset            bool
	ignorePreflightErrors sets.Set[string]
	inputReader           io.Reader
	outputWriter          io.Writer
	cfg                   *kubeadmapi.InitConfiguration
	resetCfg              *kubeadmapi.ResetConfiguration
	dryRun                bool
	cleanupTmpDir         bool
	ikniteCluster         *v1alpha1.IkniteCluster
}

// newResetOptions returns a struct ready for being used for creating cmd join flags.
func newResetOptions() *resetOptions {
	// initialize the public kubeadm config API by applying defaults
	externalCfg := &v1beta4.ResetConfiguration{}
	// Iknite: Don't prompt for confirmation
	externalCfg.Force = true
	// Apply defaults
	kubeadmScheme.Scheme.Default(externalCfg)

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}
	v1alpha1.SetDefaults_IkniteClusterSpec(ikniteConfig)

	return &resetOptions{
		kubeconfigPath:        kubeadmConstants.GetAdminKubeConfigPath(),
		externalCfg:           externalCfg,
		ikniteCfg:             ikniteConfig,
		ignorePreflightErrors: []string{"all"}, // Iknite: Ignore all preflight errors
	}
}

// newResetData returns a new resetData struct to be used for the execution of the kubeadm reset workflow.
func newResetData(
	cmd *cobra.Command, opts *resetOptions, in io.Reader, out io.Writer, allowExperimental bool,
) (*resetData, error) {
	// Validate the mixed arguments with --config and return early on errors
	if err := validation.ValidateMixedArguments(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("failed to validate mixed arguments: %w", err)
	}

	// Retrieve information from environment variables and apply them to the configuration
	if err := config.DecodeIkniteConfig(opts.ikniteCfg); err != nil {
		return nil, fmt.Errorf("failed to decode iknite config: %w", err)
	}

	ikniteCluster := &v1alpha1.IkniteCluster{}
	ikniteCluster.TypeMeta = metaV1.TypeMeta{
		Kind:       ikniteApi.IkniteClusterKind,
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}
	kubeadmScheme.Scheme.Default(ikniteCluster)

	ikniteCluster.Spec = *opts.ikniteCfg

	var (
		initCfg *kubeadmapi.InitConfiguration
		client  clientset.Interface
	)

	// Either use the config file if specified, or convert public kubeadm API to the internal ResetConfiguration and
	// validates cfg.
	resetCfg, err := configUtil.LoadOrDefaultResetConfiguration(opts.cfgPath, opts.externalCfg,
		configUtil.LoadOrDefaultConfigurationOptions{
			AllowExperimental: allowExperimental,
			SkipCRIDetect:     opts.skipCRIDetect,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to load or default reset configuration: %w", err)
	}

	dryRunFlag := cmdUtil.ValueFromFlagsOrConfig(cmd.Flags(), options.DryRun, resetCfg.DryRun,
		opts.externalCfg.DryRun).(bool)
	if dryRunFlag {
		dryRun := apiClient.NewDryRun().WithDefaultMarshalFunction().WithWriter(os.Stdout)
		dryRun.AppendReactor(dryRun.GetKubeadmConfigReactor()).
			AppendReactor(dryRun.GetKubeletConfigReactor()).
			AppendReactor(dryRun.GetKubeProxyConfigReactor())
		client = dryRun.FakeClient()
		_, err = os.Stat(opts.kubeconfigPath)
		if err == nil {
			err = dryRun.WithKubeConfigFile(opts.kubeconfigPath)
		}
	} else {
		client, err = kubeconfigUtil.ClientSetFromFile(opts.kubeconfigPath)
	}

	if err == nil {
		klog.V(1).Infof("[reset] Loaded client set from kubeconfig file: %s", opts.kubeconfigPath)
		initCfg, err = configUtil.FetchInitConfigurationFromCluster(
			client,
			nil,
			"reset",
			false,
			false,
			false,
		)
		if err != nil {
			klog.Warningf(
				"[reset] Unable to fetch the kubeadm-config ConfigMap from cluster: %v",
				err,
			)
		}
	} else {
		klog.V(1).Infof("[reset] Could not obtain a client set from the kubeconfig file: %s", opts.kubeconfigPath)
	}

	ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(
		opts.ignorePreflightErrors,
		resetCfg.IgnorePreflightErrors,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to validate ignore preflight errors: %w", err)
	}
	if initCfg != nil {
		// Also set the union of pre-flight errors to InitConfiguration, to provide a consistent view of the runtime
		// configuration:
		initCfg.NodeRegistration.IgnorePreflightErrors = sets.List(ignorePreflightErrorsSet)
	}

	criSocketPath := opts.externalCfg.CRISocket
	if criSocketPath == "" {
		criSocketPath, err = resetDetectCRISocket(resetCfg, initCfg)
		if err != nil {
			return nil, err
		}
		klog.V(1).Infof("[reset] Using specified CRI socket: %s", criSocketPath)
	}

	certificatesDir := kubeadmapiv1.DefaultCertificatesDir
	switch {
	case cmd.Flags().Changed(options.CertificatesDir): // flag is specified
		certificatesDir = opts.externalCfg.CertificatesDir
	case resetCfg.CertificatesDir != "": // configured in the ResetConfiguration
		certificatesDir = resetCfg.CertificatesDir
	case initCfg.CertificatesDir != "": // fetch from cluster
		certificatesDir = initCfg.CertificatesDir
	}

	return &resetData{
		certificatesDir:       certificatesDir,
		client:                client,
		criSocketPath:         criSocketPath,
		ignorePreflightErrors: ignorePreflightErrorsSet,
		inputReader:           in,
		outputWriter:          out,
		cfg:                   initCfg,
		resetCfg:              resetCfg,
		dryRun: cmdUtil.ValueFromFlagsOrConfig(cmd.Flags(), options.DryRun, resetCfg.DryRun,
			opts.externalCfg.DryRun).(bool),
		forceReset: cmdUtil.ValueFromFlagsOrConfig(cmd.Flags(), options.Force, resetCfg.Force,
			opts.externalCfg.Force).(bool),
		cleanupTmpDir: cmdUtil.ValueFromFlagsOrConfig(cmd.Flags(), options.CleanupTmpDir, resetCfg.CleanupTmpDir,
			opts.externalCfg.CleanupTmpDir).(bool),
		ikniteCluster: ikniteCluster,
	}, nil
}

// newCmdReset returns the "kubeadm reset" command.
func newCmdReset(in io.Reader, out io.Writer, resetOptions *resetOptions) *cobra.Command {
	if resetOptions == nil {
		resetOptions = newResetOptions()
	}
	resetRunner := workflow.NewRunner()

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Performs a best effort revert of changes made to this host by 'kubeadm init' or 'kubeadm join'",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := resetRunner.InitData(args)
			if err != nil {
				return fmt.Errorf("failed to initialize reset data: %w", err)
			}
			if _, ok := data.(*resetData); !ok {
				return errors.New("invalid data struct")
			}
			if err := resetRunner.Run(args); err != nil {
				return fmt.Errorf("failed to run reset: %w", err)
			}

			return nil
		},
	}

	AddResetFlags(cmd.Flags(), resetOptions)
	config.ConfigureClusterCommand(cmd.Flags(), resetOptions.ikniteCfg)

	// initialize the workflow runner with the list of phases
	resetRunner.AppendPhase(phases.NewPreflightPhase())
	resetRunner.AppendPhase(phases.NewRemoveETCDMemberPhase())
	resetRunner.AppendPhase(iknitePhases.NewCleanupServicePhase())
	resetRunner.AppendPhase(iknitePhases.NewCleanupNodePhase())
	resetRunner.AppendPhase(iknitePhases.NewCleanupConfigPhase())

	// sets the data builder function, that will be used by the runner
	// both when running the entire workflow or single phases
	resetRunner.SetDataInitializer(
		func(cmd *cobra.Command, args []string) (workflow.RunData, error) {
			if cmd.Flags().Lookup(options.NodeCRISocket) == nil {
				// skip CRI detection
				// assume that the command execution does not depend on CRISocket when --cri-socket flag is not set
				resetOptions.skipCRIDetect = true
			}
			data, err := newResetData(cmd, resetOptions, in, out, true)
			if err != nil {
				return nil, err
			}
			// If the flag for skipping phases was empty, use the values from config
			if len(resetRunner.Options.SkipPhases) == 0 {
				resetRunner.Options.SkipPhases = data.resetCfg.SkipPhases
			}
			return data, nil
		},
	)

	// binds the Runner to kubeadm reset command by altering
	// command help, adding --skip-phases flag and by adding phases subcommands
	resetRunner.BindToCommand(cmd)

	return cmd
}

// ResetCfg returns the ResetConfiguration.
func (r *resetData) ResetCfg() *kubeadmapi.ResetConfiguration {
	return r.resetCfg
}

// Cfg returns the InitConfiguration.
func (r *resetData) Cfg() *kubeadmapi.InitConfiguration {
	return r.cfg
}

// DryRun returns the dryRun flag.
func (r *resetData) DryRun() bool {
	return r.dryRun
}

// CleanupTmpDir returns the cleanupTmpDir flag.
func (r *resetData) CleanupTmpDir() bool {
	return r.cleanupTmpDir
}

// CertificatesDir returns the CertificatesDir.
func (r *resetData) CertificatesDir() string {
	return r.certificatesDir
}

// Client returns the Client for accessing the cluster.
func (r *resetData) Client() clientset.Interface {
	return r.client
}

// ForceReset returns the forceReset flag.
func (r *resetData) ForceReset() bool {
	return r.forceReset
}

// InputReader returns the io.reader used to read messages.
func (r *resetData) InputReader() io.Reader {
	return r.inputReader
}

// IgnorePreflightErrors returns the list of preflight errors to ignore.
func (r *resetData) IgnorePreflightErrors() sets.Set[string] {
	return r.ignorePreflightErrors
}

// CRISocketPath returns the criSocketPath.
func (r *resetData) CRISocketPath() string {
	return r.criSocketPath
}

// IkniteCluster returns the IkniteCluster.
func (r *resetData) IkniteCluster() *v1alpha1.IkniteCluster {
	return r.ikniteCluster
}
