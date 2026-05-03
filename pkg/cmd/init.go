/*
Copyright 2019 The Kubernetes Authors.

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

// cSpell:words kubeproxyconfig clientcmdapi clientcmd kubeletconfig conntrack
// cSpell: disable
import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"syscall"
	"time"
	_ "unsafe"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmScheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmApiV1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	kubeadmCmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	phases "k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/init"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	cmdUtil "k8s.io/kubernetes/cmd/kubeadm/app/cmd/util"
	componentConfigs "k8s.io/kubernetes/cmd/kubeadm/app/componentconfigs"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	certsPhase "k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	kubeconfigPhase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	configUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"

	"github.com/kaweezle/iknite/pkg/alpine"
	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	iknitePhase "github.com/kaweezle/iknite/pkg/k8s/phases/init"
	ikniteServer "github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

// initOptions defines all the init options exposed via flags by kubeadm init.
// Please note that this structure includes the public kubeadm config API, but only a subset of the options
// supported by this api will be exposed as a flag.
//
//nolint:govet // Data structure alignment matches kubeadm
type initOptions struct {
	cfgPath                 string
	skipTokenPrint          bool
	dryRun                  bool
	kubeconfigDir           string
	kubeconfigPath          string
	featureGatesString      string
	ignorePreflightErrors   []string
	bto                     *options.BootstrapTokenOptions
	externalInitCfg         *kubeadmApiV1.InitConfiguration
	externalClusterCfg      *kubeadmApiV1.ClusterConfiguration
	uploadCerts             bool
	skipCertificateKeyPrint bool
	patchesDir              string
	skipCRIDetect           bool
	ikniteCfg               *v1alpha1.IkniteClusterSpec
	kustomizeOptions        *utils.KustomizeOptions
}

const (
	// CoreDNSPhase is the name of CoreDNS sub phase in "kubeadm init".
	coreDNSPhase = "addon/coredns"

	// KubeProxyPhase is the name of kube-proxy sub phase during "kubeadm init".
	kubeProxyPhase = "addon/kube-proxy"

	// AddonPhase is the name of addon phase during "kubeadm init".
	addonPhase = "addon"
)

// function hook used for testing purposes to mock the addition of phases to the workflow runner in init command.
var addInitWorkflowPhasesFn = addInitWorkflowPhases

// HACK: This is a hack to allow the use of the unexported initOptions struct in the kubeadm codebase.
// This is needed because the kubeadm codebase uses the unexported initOptions struct in the AddInitOtherFlags function.
//
//go:linkname AddInitOtherFlags k8s.io/kubernetes/cmd/kubeadm/app/cmd.AddInitOtherFlags
func AddInitOtherFlags(flagSet *flag.FlagSet, initOptions *initOptions)

// addInitWorkflowPhases adds to the workflow runner the list of phases that should be executed when running kubeadm
// init.
func addInitWorkflowPhases(initRunner *workflow.Runner) {
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewPrepareHostPhase(), ikniteApi.Started, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewPreCleanHostPhase(), ikniteApi.Started, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewPreflightPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewCertsPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewKubeConfigPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewEtcdPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewKineControlPlanePhase(), ikniteApi.Initializing, nil))
	controlPlanePhase := phases.NewControlPlanePhase()
	controlPlanePhase.Phases = append(controlPlanePhase.Phases, iknitePhase.NewKubeVipControlPlanePhase())

	initRunner.AppendPhase(WrapPhase(controlPlanePhase, ikniteApi.Initializing, nil))
	initRunner.AppendPhase(
		WrapPhase(iknitePhase.NewKubeletStartPhase(), ikniteApi.Initializing, nil),
	)
	initRunner.AppendPhase(
		WrapPhase(phases.NewWaitControlPlanePhase(), ikniteApi.Initializing, nil),
	)
	initRunner.AppendPhase(WrapPhase(phases.NewUploadConfigPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewUploadCertsPhase(), ikniteApi.Initializing, nil))
	//nolint:gocritic // both control plane and worker
	// initRunner.AppendPhase(phases.NewMarkControlPlanePhase())
	initRunner.AppendPhase(WrapPhase(phases.NewBootstrapTokenPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewKubeletFinalizePhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewAddonPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewCopyConfigPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewMDnsPublishPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(
		WrapPhase(iknitePhase.NewKustomizeClusterPhase(), ikniteApi.Stabilizing, nil),
	)
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewServePhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewWorkloadsPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewDaemonizePhase(), ikniteApi.Stabilizing, nil))
	//nolint:gocritic // standalone node
	// initRunner.AppendPhase(phases.NewShowJoinCommandPhase())
}

// newCmdInit returns "kubeadm init" command.
//
// NB. InitOptions is exposed as parameter for allowing unit testing of the newInitOptions method, that implements all
// the command options validation logic.
//
//nolint:gocyclo // TODO: reduce more
func newCmdInit(
	out io.Writer,
	initOptions *initOptions,
	initRunner *workflow.Runner,
	alpineHost host.Host,
) *cobra.Command {
	if alpineHost == nil {
		alpineHost = host.NewDefaultHost()
	}
	if initOptions == nil {
		initOptions = newInitOptions()
	}
	if initRunner == nil {
		initRunner = workflow.NewRunner()
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Run this command in order to set up the Kubernetes control plane",
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := initRunner.InitData(args)
			if err != nil {
				return fmt.Errorf("failed to initialize init data: %w", err)
			}

			data, ok := c.(*initData)
			if !ok {
				return errors.New("invalid data struct")
			}

			log.WithField("phase", "init").
				Infof("Using Kubernetes version: %s", data.cfg.KubernetesVersion)

			return initRunner.Run(args)
		},
		Args: cobra.NoArgs,
		PostRunE: func(_ *cobra.Command, args []string) error {
			c, err := initRunner.InitData(args)
			if err != nil {
				return fmt.Errorf("failed to initialize init data in post-run: %w", err)
			}
			data, ok := c.(*initData)
			if !ok {
				return errors.New("invalid data struct")
			}
			// Stop the status server if it was started
			if shutdownErr := ikniteServer.ShutdownServer(data.statusServer); shutdownErr != nil {
				log.WithError(shutdownErr).Warn("Failed to stop iknite status server")
			}
			// Stop the kubelet process if it was started
			kubeletProcess := data.KubeletProcess()
			if kubeletProcess != nil {
				err = kubeletProcess.Signal(syscall.SIGTERM)
				if err != nil {
					return fmt.Errorf(
						"failed to terminate the kubelet process %d: %w",
						kubeletProcess.Pid(),
						err,
					)
				}
				if err = kubeletProcess.Wait(); err != nil {
					return fmt.Errorf(
						"kubelet process %d exited with error: %w",
						kubeletProcess.Pid(),
						err,
					)
				}
			}
			alpine.RemovePidFile(alpineHost, k8s.KubeletName)

			return nil
		},
	}

	// add flags to the init command.
	// init command local flags could be eventually inherited by the sub-commands automatically generated for phases
	kubeadmCmd.AddInitConfigFlags(cmd.Flags(), initOptions.externalInitCfg)
	kubeadmCmd.AddClusterConfigFlags(
		cmd.Flags(),
		initOptions.externalClusterCfg,
		&initOptions.featureGatesString,
	)

	// Keep: this is an example of how to call a method casting the unexported struct value
	// methodVal := reflect.ValueOf(kubeadmCmd.AddInitOtherFlags)
	// unexportedCastedValue := reflect.NewAt(methodVal.Type().In(1).Elem(), unsafe.Pointer(initOptions))
	// methodVal.Call([]reflect.Value{reflect.ValueOf(cmd.Flags()), unexportedCastedValue})

	AddInitOtherFlags(cmd.Flags(), initOptions)

	initOptions.bto.AddTokenFlag(cmd.Flags())
	initOptions.bto.AddTTLFlag(cmd.Flags())
	options.AddImageMetaFlags(cmd.Flags(), &initOptions.externalClusterCfg.ImageRepository)
	config.AddIkniteClusterFlags(cmd.Flags(), initOptions.ikniteCfg)
	utils.AddKustomizeOptionsFlags(cmd.Flags(), initOptions.kustomizeOptions)

	// defines additional flag that are not used by the init command but that could be eventually used
	// by the sub-commands automatically generated for phases
	initRunner.SetAdditionalFlags(func(flags *flag.FlagSet) {
		options.AddKubeConfigFlag(flags, &initOptions.kubeconfigPath)
		options.AddKubeConfigDirFlag(flags, &initOptions.kubeconfigDir)
		options.AddControlPlanExtraArgsFlags(
			flags,
			&initOptions.externalClusterCfg.APIServer.ExtraArgs,
			&initOptions.externalClusterCfg.ControllerManager.ExtraArgs,
			&initOptions.externalClusterCfg.Scheduler.ExtraArgs,
		)
	})

	// initialize the workflow runner with the list of phases
	addInitWorkflowPhasesFn(initRunner)

	// sets the data builder function, that will be used by the runner
	// both when running the entire workflow or single phases
	initRunner.SetDataInitializer(
		func(cmd *cobra.Command, args []string) (workflow.RunData, error) {
			if cmd.Flags().Lookup(options.NodeCRISocket) == nil {
				// skip CRI detection
				// assume that the command execution does not depend on CRISocket when --cri-socket flag is not set
				initOptions.skipCRIDetect = true
			}
			data, err := newInitData(cmd, args, initOptions, out, alpineHost)
			if err != nil {
				return nil, err
			}
			// If the flag for skipping phases was empty, use the values from config
			if len(initRunner.Options.SkipPhases) == 0 {
				initRunner.Options.SkipPhases = data.cfg.SkipPhases
			}

			// Skip either kine or etcd based on UseEtcd setting.
			if data.ikniteCluster.Spec.UseEtcd {
				skipPhaseIfExists(initRunner.Phases, &(initRunner.Options.SkipPhases), constants.KineBackendName, "")
				data.ikniteCluster.Spec.APIBackendDatabaseDirectory = data.cfg.Etcd.Local.DataDir
			} else {
				skipPhaseIfExists(initRunner.Phases, &(initRunner.Options.SkipPhases), constants.EtcdBackendName, "")
			}

			// force skip CoreDNS as it is injected by the kustomization
			skipPhaseIfExists(initRunner.Phases, &initRunner.Options.SkipPhases, coreDNSPhase, "")

			initRunner.Options.SkipPhases = manageSkippedAddons(
				&data.cfg.ClusterConfiguration,
				initRunner.Options.SkipPhases,
			)
			return data, nil
		},
	)

	// binds the Runner to kubeadm init command by altering
	// command help, adding --skip-phases flag and by adding phases subcommands
	initRunner.BindToCommand(cmd)

	return cmd
}

func skipPhaseIfExists(initPhases []workflow.Phase, skipPhases *[]string, phaseName, prefix string) bool {
	for i := range initPhases {
		phase := &initPhases[i]
		fullPhaseName := prefix + phase.Name
		if fullPhaseName == phaseName {
			*skipPhases = append(*skipPhases, fullPhaseName)
			return true
		}
		if len(phase.Phases) > 0 {
			if skipPhaseIfExists(phase.Phases, skipPhases, phaseName, fullPhaseName+"/") {
				return true
			}
		}
	}
	return false
}

// newInitOptions returns a struct ready for being used for creating cmd init flags.
func newInitOptions() *initOptions {
	// initialize the public kubeadm config API by applying defaults
	externalInitCfg := &kubeadmApiV1.InitConfiguration{}
	kubeadmScheme.Scheme.Default(externalInitCfg)

	externalClusterCfg := &kubeadmApiV1.ClusterConfiguration{}
	kubeadmScheme.Scheme.Default(externalClusterCfg)
	externalClusterCfg.Networking.PodSubnet = constants.PodSubnet

	// Create the options object for the bootstrap token-related flags, and override the default value for .Description
	bto := options.NewBootstrapTokenOptions()
	bto.Description = "The default bootstrap token generated by 'kubeadm init'."

	ikniteConfig := &v1alpha1.IkniteClusterSpec{}
	v1alpha1.SetDefaults_IkniteClusterSpec(ikniteConfig)

	return &initOptions{
		externalInitCfg:       externalInitCfg,
		externalClusterCfg:    externalClusterCfg,
		bto:                   bto,
		kubeconfigDir:         kubeadmConstants.KubernetesDir,
		kubeconfigPath:        kubeadmConstants.GetAdminKubeConfigPath(),
		uploadCerts:           false,
		ikniteCfg:             ikniteConfig,
		ignorePreflightErrors: []string{"all"},
		kustomizeOptions:      utils.NewKustomizeOptions(),
	}
}

// newInitData returns a new initData struct to be used for the execution of the kubeadm init workflow.
//
// This func takes care of validating initOptions passed to the command, and then it converts options into the internal
// InitConfiguration type that is used as input all the phases in the kubeadm init workflow.
//
//nolint:gocyclo // This comes from kubeadm
func newInitData(
	cmd *cobra.Command,
	_ []string,
	initOptions *initOptions,
	out io.Writer,
	alpineHost host.Host,
) (*initData, error) {
	// Re-apply defaults to the public kubeadm API (this will set only values not exposed/not set as a flags)
	kubeadmScheme.Scheme.Default(initOptions.externalInitCfg)
	kubeadmScheme.Scheme.Default(initOptions.externalClusterCfg)

	// Retrieve information from environment variables and apply them to the configuration
	if err := config.DecodeIkniteConfig(initOptions.ikniteCfg); err != nil {
		return nil, fmt.Errorf("failed to decode iknite config: %w", err)
	}

	ikniteCluster := &v1alpha1.IkniteCluster{}
	ikniteCluster.TypeMeta = metaV1.TypeMeta{
		Kind:       ikniteApi.IkniteClusterKind,
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}
	kubeadmScheme.Scheme.Default(ikniteCluster)
	ikniteCluster.Spec = *initOptions.ikniteCfg

	// Apply IkniteClusterSpec to InitConfiguration
	config.ApplyIkniteClusterSpecToClusterConfigurationV1(
		initOptions.ikniteCfg,
		initOptions.externalClusterCfg,
	)

	// Validate standalone flags values and/or combination of flags and then assigns
	// validated values to the public kubeadm config API when applicable
	var err error
	if initOptions.externalClusterCfg.FeatureGates,
		err = features.NewFeatureGate(&features.InitFeatureGates, initOptions.featureGatesString); err != nil {
		return nil, fmt.Errorf("failed to parse feature gates: %w", err)
	}

	if err = validation.ValidateMixedArguments(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("failed to validate mixed arguments: %w", err)
	}

	if err = initOptions.bto.ApplyTo(initOptions.externalInitCfg); err != nil {
		return nil, fmt.Errorf("failed to apply bootstrap token options: %w", err)
	}

	// Either use the config file if specified, or convert public kubeadm API to the internal InitConfiguration
	// and validates InitConfiguration
	cfg, err := configUtil.LoadOrDefaultInitConfiguration(
		initOptions.cfgPath,
		initOptions.externalInitCfg,
		initOptions.externalClusterCfg,
		configUtil.LoadOrDefaultConfigurationOptions{
			SkipCRIDetect: initOptions.skipCRIDetect,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load or default init configuration: %w", err)
	}

	// Set the iptables sync duration to 10 seconds instead of 30 seconds for faster restarts
	// In case the IP tables are reset (reboot, etc).
	kubeProxyConfig := cfg.ClusterConfiguration.ComponentConfigs[componentConfigs.KubeProxyGroup].Get()
	kubeProxyConfigTyped, ok := kubeProxyConfig.(*kubeproxyconfig.KubeProxyConfiguration)
	if !ok {
		return nil, errors.New("could not convert the KubeletConfiguration to a typed object")
	}
	kubeProxyConfigTyped.IPTables.SyncPeriod.Duration = 10 * time.Second
	var maxPerCore int32 = 0
	kubeProxyConfigTyped.Conntrack.MaxPerCore = &maxPerCore

	// Set FailSwapOn to false. Kubelet checks for /proc/swaps and if it exists, it will fail to start if FailSwapOn is
	// true. On Incus, the device is passed to the containers so it's ok to ignore it.
	kubeletComponentConfig, ok := cfg.ComponentConfigs[componentConfigs.KubeletGroup]
	if !ok {
		return nil, errors.New("no kubelet component config found")
	}
	kubeletConfig, ok := kubeletComponentConfig.Get().(*kubeletconfig.KubeletConfiguration)
	if !ok {
		return nil, errors.New("could not convert the KubeletConfiguration to a typed object")
	}
	failSwapOn := false
	kubeletConfig.FailSwapOn = &failSwapOn

	ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(
		initOptions.ignorePreflightErrors,
		cfg.NodeRegistration.IgnorePreflightErrors,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to validate ignore preflight errors: %w", err)
	}
	// Also set the union of pre-flight errors to InitConfiguration, to provide a consistent view of the runtime
	// configuration:
	cfg.NodeRegistration.IgnorePreflightErrors = sets.List(ignorePreflightErrorsSet)

	// override node name from the command line option
	if initOptions.externalInitCfg.NodeRegistration.Name != "" {
		cfg.NodeRegistration.Name = initOptions.externalInitCfg.NodeRegistration.Name
	}

	if err = configUtil.VerifyAPIServerBindAddress(
		cfg.LocalAPIEndpoint.AdvertiseAddress,
	); err != nil {
		return nil, fmt.Errorf("failed to verify API server bind address: %w", err)
	}
	if err = features.ValidateVersion(
		features.InitFeatureGates,
		cfg.FeatureGates,
		cfg.KubernetesVersion,
	); err != nil {
		return nil, fmt.Errorf("failed to validate version: %w", err)
	}

	// if dry running creates a temporary folder for saving kubeadm generated files
	dryRunDir := ""
	if initOptions.dryRun || cfg.DryRun {
		// the KUBEADM_INIT_DRYRUN_DIR environment variable allows overriding the dry-run temporary
		// directory from the command line. This makes it possible to run "kubeadm init" integration
		// tests without root.
		if dryRunDir, err = kubeadmConstants.CreateTempDir(os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
			"kubeadm-init-dryrun"); err != nil {
			return nil, fmt.Errorf("couldn't create a temporary directory: %w", err)
		}
	}

	// Checks if an external CA is provided by the user (when the CA Cert is present but the CA Key is not)
	externalCA, err := certsPhase.UsingExternalCA(&cfg.ClusterConfiguration)
	if externalCA {
		// In case the certificates signed by CA (that should be provided by the user) are missing or invalid,
		// returns, because kubeadm can't regenerate them without the CA Key
		if err != nil {
			return nil, fmt.Errorf("invalid or incomplete external CA: %w", err)
		}

		// Validate that also the required kubeconfig files exists and are invalid, because
		// kubeadm can't regenerate them without the CA Key
		kubeconfigDir := initOptions.kubeconfigDir
		if err = kubeconfigPhase.ValidateKubeconfigsForExternalCA(kubeconfigDir, cfg); err != nil {
			return nil, fmt.Errorf("failed to validate kubeconfigs for external CA: %w", err)
		}
	}

	// Checks if an external Front-Proxy CA is provided by the user (when the Front-Proxy CA Cert is present but the
	// Front-Proxy CA Key is not).
	externalFrontProxyCA, err := certsPhase.UsingExternalFrontProxyCA(&cfg.ClusterConfiguration)
	if externalFrontProxyCA {
		// In case the certificates signed by Front-Proxy CA (that should be provided by the user) are missing or
		// invalid, returns, because kubeadm can't regenerate them without the Front-Proxy CA Key
		if err != nil {
			return nil, fmt.Errorf("invalid or incomplete external front-proxy CA: %w", err)
		}
	}

	if initOptions.uploadCerts && (externalCA || externalFrontProxyCA) {
		return nil, errors.New(
			"can't use upload-certs with an external CA or an external front-proxy CA",
		)
	}

	// Apply ikniteCluster spec to the internal InitConfiguration
	config.ApplyIkniteClusterSpecToInitConfiguration(&(ikniteCluster.Spec), cfg)

	return &initData{
		cfg:                     cfg,
		certificatesDir:         cfg.CertificatesDir,
		skipTokenPrint:          initOptions.skipTokenPrint,
		dryRunDir:               dryRunDir,
		kubeconfigDir:           initOptions.kubeconfigDir,
		kubeconfigPath:          initOptions.kubeconfigPath,
		ignorePreflightErrors:   ignorePreflightErrorsSet,
		externalCA:              externalCA,
		outputWriter:            out,
		uploadCerts:             initOptions.uploadCerts,
		skipCertificateKeyPrint: initOptions.skipCertificateKeyPrint,
		patchesDir:              initOptions.patchesDir,
		ikniteCluster:           ikniteCluster,
		ctx:                     cmd.Context(),
		kustomizeOptions:        initOptions.kustomizeOptions,
		dryRun: cmdUtil.ValueFromFlagsOrConfig( //nolint:errcheck,forcetypeassert // default value is false
			cmd.Flags(),
			options.DryRun,
			cfg.DryRun,
			initOptions.dryRun).(bool),
		alpineHost: alpineHost,
	}, nil
}

// manageSkippedAddons syncs proxy and DNS "Disabled" status and skipPhases.
func manageSkippedAddons(cfg *kubeadmApi.ClusterConfiguration, skipPhases []string) []string {
	var (
		skipDNSPhase   = false
		skipProxyPhase = false
	)
	// If the DNS or Proxy addons are disabled, skip the corresponding phase.
	// Alternatively, update the proxy and DNS "Disabled" status based on skipped addon phases.
	if slices.Contains(skipPhases, addonPhase) {
		skipDNSPhase = true
		skipProxyPhase = true
		cfg.DNS.Disabled = true
		cfg.Proxy.Disabled = true
	}
	if slices.Contains(skipPhases, coreDNSPhase) {
		skipDNSPhase = true
		cfg.DNS.Disabled = true
	}
	if slices.Contains(skipPhases, kubeProxyPhase) {
		skipProxyPhase = true
		cfg.Proxy.Disabled = true
	}
	if cfg.DNS.Disabled && !skipDNSPhase {
		skipPhases = append(skipPhases, coreDNSPhase)
	}
	if cfg.Proxy.Disabled && !skipProxyPhase {
		skipPhases = append(skipPhases, kubeProxyPhase)
	}
	return skipPhases
}

func PhaseName(
	p workflow.Phase, //nolint:gocritic // matching kubeadm style
	parentPhases *[]workflow.Phase,
) string {
	if len(*parentPhases) == 0 {
		return p.Name
	}
	parentPhaseName := (*parentPhases)[len(*parentPhases)-1]
	grandParentPhases := (*parentPhases)[:len(*parentPhases)-1]
	return fmt.Sprintf("%s/%s", PhaseName(parentPhaseName, &grandParentPhases), p.Name)
}

//nolint:gocritic // matching kubeadm style
func WrapPhase(
	p workflow.Phase,
	state ikniteApi.ClusterState,
	parentPhases *[]workflow.Phase,
) workflow.Phase {
	var newRun func(c workflow.RunData) error
	var newChildPhases []workflow.Phase
	if parentPhases == nil {
		parentPhases = &[]workflow.Phase{}
	}

	if p.Run != nil {
		oldRun := p.Run
		newRun = func(c workflow.RunData) error {
			// Cast the data to the expected type
			data, ok := c.(*initData)
			if !ok {
				return fmt.Errorf("phase %q invoked with an invalid data struct", p.Name)
			}
			phaseName := PhaseName(p, parentPhases)
			data.UpdateIkniteCluster(state, phaseName, nil, nil)

			log.WithFields(log.Fields{
				"phase": phaseName,
				"state": state.String(),
			}).Infof("Running phase %s...", phaseName)

			return oldRun(c)
		}
	}
	if p.Phases != nil {
		newChildPhases = make([]workflow.Phase, 0, len(p.Phases))
		newParentPhases := append(*parentPhases, p)
		for _, childPhase := range p.Phases {
			newChildPhases = append(newChildPhases, WrapPhase(childPhase, state, &newParentPhases))
		}
	}

	p.Run = newRun
	p.Phases = newChildPhases
	return p
}
