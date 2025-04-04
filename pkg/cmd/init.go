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

// cSpell:words kubeproxyconfig
// cSpell: disable
import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pion/mdns"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"

	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmScheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmApiV1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta4"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
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
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	_ "unsafe"

	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/k8s"
	iknitePhase "github.com/kaweezle/iknite/pkg/k8s/phases/init"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeadmCmd "k8s.io/kubernetes/cmd/kubeadm/app/cmd"
)

// cSpell: enable

// initOptions defines all the init options exposed via flags by kubeadm init.
// Please note that this structure includes the public kubeadm config API, but only a subset of the options
// supported by this api will be exposed as a flag.
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
}

const (
	// CoreDNSPhase is the name of CoreDNS sub phase in "kubeadm init"
	coreDNSPhase = "addon/coredns"

	// KubeProxyPhase is the name of kube-proxy sub phase during "kubeadm init"
	kubeProxyPhase = "addon/kube-proxy"

	// AddonPhase is the name of addon phase during "kubeadm init"
	addonPhase = "addon"
)

// compile-time assert that the local data object satisfies the phases data interface.
var _ phases.InitData = &initData{}

// initData defines all the runtime information used when running the kubeadm init workflow;
// this data is shared across all the phases that are included in the workflow.
type initData struct {
	cfg                         *kubeadmApi.InitConfiguration
	skipTokenPrint              bool
	dryRun                      bool
	kubeconfigDir               string
	kubeconfigPath              string
	ignorePreflightErrors       sets.Set[string]
	certificatesDir             string
	dryRunDir                   string
	externalCA                  bool
	client                      clientset.Interface
	outputWriter                io.Writer
	uploadCerts                 bool
	skipCertificateKeyPrint     bool
	patchesDir                  string
	adminKubeConfigBootstrapped bool
	ikniteCluster               *v1alpha1.IkniteCluster
	kubeletCmd                  *exec.Cmd
	mdnsConn                    *mdns.Conn
	ctx                         context.Context
	ctxCancel                   context.CancelFunc
}

// HACK: This is a hack to allow the use of the unexported initOptions struct in the kubeadm codebase.
// This is needed because the kubeadm codebase uses the unexported initOptions struct in the AddInitOtherFlags function.
//
//go:linkname AddInitOtherFlags k8s.io/kubernetes/cmd/kubeadm/app/cmd.AddInitOtherFlags
func AddInitOtherFlags(flagSet *flag.FlagSet, initOptions *initOptions)

//go:linkname getDryRunClient k8s.io/kubernetes/cmd/kubeadm/app/cmd.getDryRunClient
func getDryRunClient(d *initData) (clientset.Interface, error)

// newCmdInit returns "kubeadm init" command.
// NB. initOptions is exposed as parameter for allowing unit testing of
// the newInitOptions method, that implements all the command options validation logic
func newCmdInit(out io.Writer, initOptions *initOptions) *cobra.Command {
	if initOptions == nil {
		initOptions = newInitOptions()
	}
	initRunner := workflow.NewRunner()

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Run this command in order to set up the Kubernetes control plane",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := initRunner.InitData(args)
			if err != nil {
				return err
			}

			data, ok := c.(*initData)
			if !ok {
				return errors.New("invalid data struct")
			}

			fmt.Printf("[init] Using Kubernetes version: %s\n", data.cfg.KubernetesVersion)

			return initRunner.Run(args)
		},
		Args: cobra.NoArgs,
		PostRunE: func(cmd *cobra.Command, args []string) error {
			c, err := initRunner.InitData(args)
			if err != nil {
				return err
			}
			data, ok := c.(*initData)
			if !ok {
				return errors.New("invalid data struct")
			}
			// Stop the kubelet process if it was started
			kubeletCmd := data.KubeletCmd()
			if kubeletCmd != nil {
				err = kubeletCmd.Process.Signal(syscall.SIGTERM)
				if err != nil {
					return errors.Wrapf(err, "failed to terminate the kubelet process %d", kubeletCmd.Process.Pid)
				}
				kubeletCmd.Wait()
			}
			k8s.RemovePidFiles()

			return nil
		},
	}

	// add flags to the init command.
	// init command local flags could be eventually inherited by the sub-commands automatically generated for phases
	kubeadmCmd.AddInitConfigFlags(cmd.Flags(), initOptions.externalInitCfg)
	kubeadmCmd.AddClusterConfigFlags(cmd.Flags(), initOptions.externalClusterCfg, &initOptions.featureGatesString)

	// Keep: this is an example of how to call a method casting the unexported struct value
	// methodVal := reflect.ValueOf(kubeadmCmd.AddInitOtherFlags)
	// unexportedCastedValue := reflect.NewAt(methodVal.Type().In(1).Elem(), unsafe.Pointer(initOptions))
	// methodVal.Call([]reflect.Value{reflect.ValueOf(cmd.Flags()), unexportedCastedValue})

	AddInitOtherFlags(cmd.Flags(), initOptions)

	initOptions.bto.AddTokenFlag(cmd.Flags())
	initOptions.bto.AddTTLFlag(cmd.Flags())
	options.AddImageMetaFlags(cmd.Flags(), &initOptions.externalClusterCfg.ImageRepository)
	config.ConfigureClusterCommand(cmd.Flags(), initOptions.ikniteCfg)

	// defines additional flag that are not used by the init command but that could be eventually used
	// by the sub-commands automatically generated for phases
	initRunner.SetAdditionalFlags(func(flags *flag.FlagSet) {
		options.AddKubeConfigFlag(flags, &initOptions.kubeconfigPath)
		options.AddKubeConfigDirFlag(flags, &initOptions.kubeconfigDir)
		options.AddControlPlanExtraArgsFlags(flags, &initOptions.externalClusterCfg.APIServer.ExtraArgs, &initOptions.externalClusterCfg.ControllerManager.ExtraArgs, &initOptions.externalClusterCfg.Scheduler.ExtraArgs)
	})

	// initialize the workflow runner with the list of phases
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewPrepareHostPhase(), ikniteApi.Started, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewPreCleanHostPhase(), ikniteApi.Started, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewPreflightPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewCertsPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewKubeConfigPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewEtcdPhase(), ikniteApi.Initializing, nil))
	controlPlanePhase := phases.NewControlPlanePhase()
	controlPlanePhase.Phases = append(controlPlanePhase.Phases, iknitePhase.NewKubeVipControlPlanePhase())

	initRunner.AppendPhase(WrapPhase(controlPlanePhase, ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewKubeletStartPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewWaitControlPlanePhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewUploadConfigPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewUploadCertsPhase(), ikniteApi.Initializing, nil))
	// initRunner.AppendPhase(phases.NewMarkControlPlanePhase())
	initRunner.AppendPhase(WrapPhase(phases.NewBootstrapTokenPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewKubeletFinalizePhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(phases.NewAddonPhase(), ikniteApi.Initializing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewMDnsPublishPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewKustomizeClusterPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewWorkloadsPhase(), ikniteApi.Stabilizing, nil))
	initRunner.AppendPhase(WrapPhase(iknitePhase.NewDaemonizePhase(), ikniteApi.Stabilizing, nil))
	// initRunner.AppendPhase(phases.NewShowJoinCommandPhase())

	// sets the data builder function, that will be used by the runner
	// both when running the entire workflow or single phases
	initRunner.SetDataInitializer(func(cmd *cobra.Command, args []string) (workflow.RunData, error) {
		if cmd.Flags().Lookup(options.NodeCRISocket) == nil {
			// skip CRI detection
			// assume that the command execution does not depend on CRISocket when --cri-socket flag is not set
			initOptions.skipCRIDetect = true
		}
		data, err := newInitData(cmd, args, initOptions, out)
		if err != nil {
			return nil, err
		}
		// If the flag for skipping phases was empty, use the values from config
		if len(initRunner.Options.SkipPhases) == 0 {
			initRunner.Options.SkipPhases = data.cfg.SkipPhases
		}

		initRunner.Options.SkipPhases = manageSkippedAddons(&data.cfg.ClusterConfiguration, initRunner.Options.SkipPhases)
		return data, nil
	})

	// binds the Runner to kubeadm init command by altering
	// command help, adding --skip-phases flag and by adding phases subcommands
	initRunner.BindToCommand(cmd)

	return cmd
}

// newInitOptions returns a struct ready for being used for creating cmd init flags.
func newInitOptions() *initOptions {
	// initialize the public kubeadm config API by applying defaults
	externalInitCfg := &kubeadmApiV1.InitConfiguration{}
	kubeadmScheme.Scheme.Default(externalInitCfg)
	externalInitCfg.SkipPhases = []string{coreDNSPhase}

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
	}
}

// newInitData returns a new initData struct to be used for the execution of the kubeadm init workflow.
// This func takes care of validating initOptions passed to the command, and then it converts
// options into the internal InitConfiguration type that is used as input all the phases in the kubeadm init workflow
func newInitData(cmd *cobra.Command, _ []string, initOptions *initOptions, out io.Writer) (*initData, error) {
	// Re-apply defaults to the public kubeadm API (this will set only values not exposed/not set as a flags)
	kubeadmScheme.Scheme.Default(initOptions.externalInitCfg)
	kubeadmScheme.Scheme.Default(initOptions.externalClusterCfg)

	// Retrieve information from environment variables and apply them to the configuration
	config.DecodeIkniteConfig(initOptions.ikniteCfg)

	ikniteCluster := &v1alpha1.IkniteCluster{}
	ikniteCluster.TypeMeta = metaV1.TypeMeta{
		Kind:       ikniteApi.IkniteClusterKind,
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}
	kubeadmScheme.Scheme.Default(ikniteCluster)
	ikniteCluster.Spec = *initOptions.ikniteCfg

	// Validate standalone flags values and/or combination of flags and then assigns
	// validated values to the public kubeadm config API when applicable
	var err error
	if initOptions.externalClusterCfg.FeatureGates, err = features.NewFeatureGate(&features.InitFeatureGates, initOptions.featureGatesString); err != nil {
		return nil, err
	}

	if err = validation.ValidateMixedArguments(cmd.Flags()); err != nil {
		return nil, err
	}

	if err = initOptions.bto.ApplyTo(initOptions.externalInitCfg); err != nil {
		return nil, err
	}

	// Either use the config file if specified, or convert public kubeadm API to the internal InitConfiguration
	// and validates InitConfiguration
	cfg, err := configUtil.LoadOrDefaultInitConfiguration(initOptions.cfgPath, initOptions.externalInitCfg, initOptions.externalClusterCfg, configUtil.LoadOrDefaultConfigurationOptions{
		SkipCRIDetect: initOptions.skipCRIDetect,
	})
	if err != nil {
		return nil, err
	}

	// Set the iptables sync duration to 10 seconds instead of 30 seconds for faster restarts
	// In case the IP tables are reset (reboot, etc).
	kubeProxyConfig := cfg.ClusterConfiguration.ComponentConfigs[componentConfigs.KubeProxyGroup].Get()
	kubeProxyConfigTyped, ok := kubeProxyConfig.(*kubeproxyconfig.KubeProxyConfiguration)
	if !ok {
		return nil, errors.New("could not convert the KubeletConfiguration to a typed object")
	}
	kubeProxyConfigTyped.IPTables.SyncPeriod.Duration = 10 * time.Second

	ignorePreflightErrorsSet, err := validation.ValidateIgnorePreflightErrors(initOptions.ignorePreflightErrors, cfg.NodeRegistration.IgnorePreflightErrors)
	if err != nil {
		return nil, err
	}
	// Also set the union of pre-flight errors to InitConfiguration, to provide a consistent view of the runtime configuration:
	cfg.NodeRegistration.IgnorePreflightErrors = sets.List(ignorePreflightErrorsSet)

	// override node name from the command line option
	if initOptions.externalInitCfg.NodeRegistration.Name != "" {
		cfg.NodeRegistration.Name = initOptions.externalInitCfg.NodeRegistration.Name
	}

	if err := configUtil.VerifyAPIServerBindAddress(cfg.LocalAPIEndpoint.AdvertiseAddress); err != nil {
		return nil, err
	}
	if err := features.ValidateVersion(features.InitFeatureGates, cfg.FeatureGates, cfg.KubernetesVersion); err != nil {
		return nil, err
	}

	// if dry running creates a temporary folder for saving kubeadm generated files
	dryRunDir := ""
	if initOptions.dryRun || cfg.DryRun {
		// the KUBEADM_INIT_DRYRUN_DIR environment variable allows overriding the dry-run temporary
		// directory from the command line. This makes it possible to run "kubeadm init" integration
		// tests without root.
		if dryRunDir, err = kubeadmConstants.CreateTempDir(os.Getenv("KUBEADM_INIT_DRYRUN_DIR"), "kubeadm-init-dryrun"); err != nil {
			return nil, errors.Wrap(err, "couldn't create a temporary directory")
		}
	}

	// Checks if an external CA is provided by the user (when the CA Cert is present but the CA Key is not)
	externalCA, err := certsPhase.UsingExternalCA(&cfg.ClusterConfiguration)
	if externalCA {
		// In case the certificates signed by CA (that should be provided by the user) are missing or invalid,
		// returns, because kubeadm can't regenerate them without the CA Key
		if err != nil {
			return nil, errors.Wrapf(err, "invalid or incomplete external CA")
		}

		// Validate that also the required kubeconfig files exists and are invalid, because
		// kubeadm can't regenerate them without the CA Key
		kubeconfigDir := initOptions.kubeconfigDir
		if err := kubeconfigPhase.ValidateKubeconfigsForExternalCA(kubeconfigDir, cfg); err != nil {
			return nil, err
		}
	}

	// Checks if an external Front-Proxy CA is provided by the user (when the Front-Proxy CA Cert is present but the Front-Proxy CA Key is not)
	externalFrontProxyCA, err := certsPhase.UsingExternalFrontProxyCA(&cfg.ClusterConfiguration)
	if externalFrontProxyCA {
		// In case the certificates signed by Front-Proxy CA (that should be provided by the user) are missing or invalid,
		// returns, because kubeadm can't regenerate them without the Front-Proxy CA Key
		if err != nil {
			return nil, errors.Wrapf(err, "invalid or incomplete external front-proxy CA")
		}
	}

	if initOptions.uploadCerts && (externalCA || externalFrontProxyCA) {
		return nil, errors.New("can't use upload-certs with an external CA or an external front-proxy CA")
	}

	// Apply the IkniteConfig to the InitConfiguration
	cfg.KubernetesVersion = fmt.Sprintf("v%s", initOptions.ikniteCfg.KubernetesVersion)
	if initOptions.ikniteCfg.DomainName != "" {
		cfg.ClusterConfiguration.ControlPlaneEndpoint = initOptions.ikniteCfg.DomainName
		cfg.ControlPlaneEndpoint = initOptions.ikniteCfg.DomainName
	}
	// Apply configured IP to the configuration
	ips := initOptions.ikniteCfg.Ip.String()
	cfg.LocalAPIEndpoint.AdvertiseAddress = ips
	arg := &kubeadmApi.Arg{Name: "node-ip", Value: ips}
	cfg.NodeRegistration.KubeletExtraArgs = append(cfg.NodeRegistration.KubeletExtraArgs, *arg)

	ctx, cancel := context.WithCancel(context.Background())

	return &initData{
		cfg:                     cfg,
		certificatesDir:         cfg.CertificatesDir,
		skipTokenPrint:          initOptions.skipTokenPrint,
		dryRun:                  cmdUtil.ValueFromFlagsOrConfig(cmd.Flags(), options.DryRun, cfg.DryRun, initOptions.dryRun).(bool),
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
		ctx:                     ctx,
		ctxCancel:               cancel,
	}, nil
}

// UploadCerts returns UploadCerts flag.
func (d *initData) UploadCerts() bool {
	return d.uploadCerts
}

// CertificateKey returns the key used to encrypt the certs.
func (d *initData) CertificateKey() string {
	return d.cfg.CertificateKey
}

// SetCertificateKey set the key used to encrypt the certs.
func (d *initData) SetCertificateKey(key string) {
	d.cfg.CertificateKey = key
}

// SkipCertificateKeyPrint returns the skipCertificateKeyPrint flag.
func (d *initData) SkipCertificateKeyPrint() bool {
	return d.skipCertificateKeyPrint
}

// Cfg returns initConfiguration.
func (d *initData) Cfg() *kubeadmApi.InitConfiguration {
	return d.cfg
}

// DryRun returns the DryRun flag.
func (d *initData) DryRun() bool {
	return d.dryRun
}

// SkipTokenPrint returns the SkipTokenPrint flag.
func (d *initData) SkipTokenPrint() bool {
	return d.skipTokenPrint
}

// IgnorePreflightErrors returns the IgnorePreflightErrors flag.
func (d *initData) IgnorePreflightErrors() sets.Set[string] {
	return d.ignorePreflightErrors
}

// CertificateWriteDir returns the path to the certificate folder or the temporary folder path in case of DryRun.
func (d *initData) CertificateWriteDir() string {
	if d.dryRun {
		return d.dryRunDir
	}
	return d.certificatesDir
}

// CertificateDir returns the CertificateDir as originally specified by the user.
func (d *initData) CertificateDir() string {
	return d.certificatesDir
}

// KubeConfigDir returns the path of the Kubernetes configuration folder or the temporary folder path in case of DryRun.
func (d *initData) KubeConfigDir() string {
	if d.dryRun {
		return d.dryRunDir
	}
	return d.kubeconfigDir
}

// KubeConfigPath returns the path to the kubeconfig file to use for connecting to Kubernetes
func (d *initData) KubeConfigPath() string {
	if d.dryRun {
		d.kubeconfigPath = filepath.Join(d.dryRunDir, kubeadmConstants.AdminKubeConfigFileName)
	}
	return d.kubeconfigPath
}

// ManifestDir returns the path where manifest should be stored or the temporary folder path in case of DryRun.
func (d *initData) ManifestDir() string {
	if d.dryRun {
		return d.dryRunDir
	}
	return kubeadmConstants.GetStaticPodDirectory()
}

// KubeletDir returns path of the kubelet configuration folder or the temporary folder in case of DryRun.
func (d *initData) KubeletDir() string {
	if d.dryRun {
		return d.dryRunDir
	}
	return kubeadmConstants.KubeletRunDirectory
}

// ExternalCA returns true if an external CA is provided by the user.
func (d *initData) ExternalCA() bool {
	return d.externalCA
}

// OutputWriter returns the io.Writer used to write output to by this command.
func (d *initData) OutputWriter() io.Writer {
	return d.outputWriter
}

// Client returns a Kubernetes client to be used by kubeadm.
// This function is implemented as a singleton, thus avoiding to recreate the client when it is used by different phases.
// Important. This function must be called after the admin.conf kubeconfig file is created.
func (d *initData) Client() (clientset.Interface, error) {
	var err error
	if d.client == nil {
		if d.dryRun {
			d.client, err = getDryRunClient(d)
			if err != nil {
				return nil, err
			}
		} else { // Use a real client
			isDefaultKubeConfigPath := d.KubeConfigPath() == kubeadmConstants.GetAdminKubeConfigPath()
			// Only bootstrap the admin.conf if it's used by the user (i.e. --kubeconfig has its default value)
			// and if the bootstrapping was not already done
			if !d.adminKubeConfigBootstrapped && isDefaultKubeConfigPath {
				// Call EnsureAdminClusterRoleBinding() to obtain a working client from admin.conf.
				d.client, err = kubeconfigPhase.EnsureAdminClusterRoleBinding(kubeadmConstants.KubernetesDir, nil)
				if err != nil {
					return nil, errors.Wrapf(err, "could not bootstrap the admin user in file %s", kubeadmConstants.AdminKubeConfigFileName)
				}
				d.adminKubeConfigBootstrapped = true
			} else {
				// Alternatively, just load the config pointed at the --kubeconfig path
				d.client, err = kubeConfigUtil.ClientSetFromFile(d.KubeConfigPath())
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return d.client, nil
}

// ClientWithoutBootstrap returns a dry-run client or a regular client from admin.conf.
// Unlike Client(), it does not call EnsureAdminClusterRoleBinding() or sets d.client.
// This means the client only has anonymous permissions and does not persist in initData.
func (d *initData) ClientWithoutBootstrap() (clientset.Interface, error) {
	var (
		client clientset.Interface
		err    error
	)
	if d.dryRun {
		client, err = getDryRunClient(d)
		if err != nil {
			return nil, err
		}
	} else { // Use a real client
		client, err = kubeConfigUtil.ClientSetFromFile(d.KubeConfigPath())
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Tokens returns an array of token strings.
func (d *initData) Tokens() []string {
	tokens := []string{}
	for _, bt := range d.cfg.BootstrapTokens {
		tokens = append(tokens, bt.Token.String())
	}
	return tokens
}

// PatchesDir returns the folder where patches for components are stored
func (d *initData) PatchesDir() string {
	// If provided, make the flag value override the one in config.
	if len(d.patchesDir) > 0 {
		return d.patchesDir
	}
	if d.cfg.Patches != nil {
		return d.cfg.Patches.Directory
	}
	return ""
}

// manageSkippedAddons syncs proxy and DNS "Disabled" status and skipPhases.
func manageSkippedAddons(cfg *kubeadmApi.ClusterConfiguration, skipPhases []string) []string {
	var (
		skipDNSPhase   = false
		skipProxyPhase = false
	)
	// If the DNS or Proxy addons are disabled, skip the corresponding phase.
	// Alternatively, update the proxy and DNS "Disabled" status based on skipped addon phases.
	if isPhaseInSkipPhases(addonPhase, skipPhases) {
		skipDNSPhase = true
		skipProxyPhase = true
		cfg.DNS.Disabled = true
		cfg.Proxy.Disabled = true
	}
	if isPhaseInSkipPhases(coreDNSPhase, skipPhases) {
		skipDNSPhase = true
		cfg.DNS.Disabled = true
	}
	if isPhaseInSkipPhases(kubeProxyPhase, skipPhases) {
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

func isPhaseInSkipPhases(phase string, skipPhases []string) bool {
	for _, item := range skipPhases {
		if item == phase {
			return true
		}
	}
	return false
}

func (d *initData) IkniteCluster() *v1alpha1.IkniteCluster {
	return d.ikniteCluster
}

func (d *initData) KubeletCmd() *exec.Cmd {
	return d.kubeletCmd
}

func (d *initData) SetKubeletCmd(cmd *exec.Cmd) {
	d.kubeletCmd = cmd
}

func (d *initData) SetMDnsConn(conn *mdns.Conn) {
	d.mdnsConn = conn
}

func (d *initData) MDnsConn() *mdns.Conn {
	return d.mdnsConn
}

func (d *initData) ContextWithCancel() (context.Context, context.CancelFunc) {
	return d.ctx, d.ctxCancel
}

func PhaseName(p workflow.Phase, parentPhases *[]workflow.Phase) string {
	if len(*parentPhases) == 0 {
		return p.Name
	}
	parentPhaseName := (*parentPhases)[len(*parentPhases)-1]
	grandParentPhases := (*parentPhases)[:len(*parentPhases)-1]
	return fmt.Sprintf("%s/%s", PhaseName(parentPhaseName, &grandParentPhases), p.Name)
}

func WrapPhase(p workflow.Phase, state ikniteApi.ClusterState, parentPhases *[]workflow.Phase) workflow.Phase {
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
			data.IkniteCluster().Update(state, phaseName, nil, nil)
			log.WithFields(log.Fields{
				"phase": phaseName,
				"state": state.String(),
			}).Infof("Running phase %s...", phaseName)

			return oldRun(c)
		}
	}
	if p.Phases != nil {
		newChildPhases = []workflow.Phase{}
		newParentPhases := append(*parentPhases, p)
		for _, childPhase := range p.Phases {
			newChildPhases = append(newChildPhases, WrapPhase(childPhase, state, &newParentPhases))
		}
	}

	p.Run = newRun
	p.Phases = newChildPhases
	return p
}
