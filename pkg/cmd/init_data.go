// cSpell: words clientcmdapi clientcmd apimachinery wrapcheck genericclioptions errgroup
package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	_ "unsafe"

	"github.com/pion/mdns"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientset "k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeconfigPhase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	iknitePhase "github.com/kaweezle/iknite/pkg/k8s/phases/init"
	ikniteServer "github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
)

// initData defines all the runtime information used when running the kubeadm init workflow;
// this data is shared across all the phases that are included in the workflow.
//
//nolint:govet // Data structure alignment matches kubeadm
type initData struct {
	cfg                         *kubeadmApi.InitConfiguration
	skipTokenPrint              bool
	dryRun                      bool
	kubeconfig                  *clientcmdapi.Config
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
	kubeletProcess              host.Process
	mdnsConn                    *mdns.Conn
	statusServer                *ikniteServer.IkniteServer
	ctx                         context.Context //nolint:containedctx // passed around but not stored
	kustomizeOptions            *utils.KustomizeOptions
	alpineHost                  host.Host
	clientGetter                genericclioptions.RESTClientGetter
	errGroup                    errgroup.Group
}

// compile-time assert that the local data object satisfies the phases data interface.
var _ iknitePhase.IkniteInitData = (*initData)(nil)

// function hooks used for testing error paths around external dependencies.
var (
	ensureAdminClusterRoleBinding = kubeconfigPhase.EnsureAdminClusterRoleBinding
	closeMDNSConn                 = func(conn *mdns.Conn) error { return conn.Close() }
	shutdownStatusServer          = func(srv *ikniteServer.IkniteServer) error { return srv.Shutdown() }
)

//go:linkname getDryRunClient k8s.io/kubernetes/cmd/kubeadm/app/cmd.getDryRunClient
func getDryRunClient(d *initData) (clientset.Interface, error)

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

// KubeConfig returns a kubeconfig after loading it from KubeConfigPath().
func (d *initData) KubeConfig() (*clientcmdapi.Config, error) {
	if d.kubeconfig != nil {
		return d.kubeconfig, nil
	}

	var err error
	d.kubeconfig, err = k8s.LoadFromFile(d.Host(), d.KubeConfigPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from file: %w", err)
	}

	return d.kubeconfig, nil
}

// KubeConfigDir returns the path of the Kubernetes configuration folder or the temporary folder path in case of DryRun.
func (d *initData) KubeConfigDir() string {
	if d.dryRun {
		return d.dryRunDir
	}
	return d.kubeconfigDir
}

// KubeConfigPath returns the path to the kubeconfig file to use for connecting to Kubernetes.
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

func (d *initData) RESTClientGetter() (genericclioptions.RESTClientGetter, error) {
	if d.clientGetter != nil {
		return d.clientGetter, nil
	}
	kubeConfig, err := d.KubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for REST client getter: %w", err)
	}
	d.clientGetter = k8s.NewClientFromConfig(kubeConfig)
	return d.clientGetter, nil
}

// Client returns a Kubernetes client to be used by kubeadm.
//
// This function is implemented as a singleton, thus avoiding to recreate the client when it is used by different
// phases.
//
// Important. This function must be called after the admin.conf kubeconfig file is created.
func (d *initData) Client() (clientset.Interface, error) {
	var err error
	if d.client != nil {
		return d.client, nil
	}
	if d.dryRun {
		return getDryRunClient(d)
	}
	// Use a real client
	isDefaultKubeConfigPath := d.KubeConfigPath() == kubeadmConstants.GetAdminKubeConfigPath()
	// Only bootstrap the admin.conf if it's used by the user (i.e. --kubeconfig has its default value)
	// and if the bootstrapping was not already done
	if !d.adminKubeConfigBootstrapped && isDefaultKubeConfigPath {
		// Call EnsureAdminClusterRoleBinding() to obtain a working client from admin.conf.
		d.client, err = ensureAdminClusterRoleBinding(
			kubeadmConstants.KubernetesDir,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("could not bootstrap the admin user in file %s: %w",
				kubeadmConstants.AdminKubeConfigFileName, err)
		}
		d.adminKubeConfigBootstrapped = true
	} else {
		// Alternatively, just load the config pointed at the --kubeconfig path
		getter, err := d.RESTClientGetter()
		if err != nil {
			return nil, fmt.Errorf("failed to get REST client getter: %w", err)
		}

		d.client, err = k8s.ClientSet(getter)
		if err != nil {
			return nil, fmt.Errorf("failed to create client set from file: %w", err)
		}
	}
	return d.client, nil
}

// WaitControlPlaneClient returns a basic client used for the purpose of waiting
// for control plane components to report 'ok' on their respective health check endpoints.
// It uses the admin.conf as the base, but modifies it to point at the local API server instead
// of the control plane endpoint.
func (d *initData) WaitControlPlaneClient() (clientset.Interface, error) {
	original, err := d.KubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig for wait control plane: %w", err)
	}
	kubeConfig := original.DeepCopy() // avoid mutating the original kubeconfig in case it's used elsewhere
	for _, v := range kubeConfig.Clusters {
		v.Server = fmt.Sprintf("https://%s",
			net.JoinHostPort(
				d.Cfg().LocalAPIEndpoint.AdvertiseAddress,
				strconv.Itoa(int(d.Cfg().LocalAPIEndpoint.BindPort)),
			),
		)
	}
	client, err := kubeConfigUtil.ToClientSet(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client set from config: %w", err)
	}
	return client, nil
}

// ClientWithoutBootstrap returns a dry-run client or a regular client from admin.conf.
// Unlike Client(), it does not call EnsureAdminClusterRoleBinding() or sets d.client.
// This means the client only has anonymous permissions and does not persist in initData.
func (d *initData) ClientWithoutBootstrap() (clientset.Interface, error) {
	if d.dryRun {
		return getDryRunClient(d)
	}
	return k8s.ClientSetFromFile(d.Host(), d.KubeConfigPath()) //nolint:wrapcheck // on-purpose
}

// Tokens returns an array of token strings.
func (d *initData) Tokens() []string {
	tokens := make([]string, 0, len(d.cfg.BootstrapTokens))
	for _, bt := range d.cfg.BootstrapTokens {
		tokens = append(tokens, bt.Token.String())
	}
	return tokens
}

// PatchesDir returns the folder where patches for components are stored.
func (d *initData) PatchesDir() string {
	// If provided, make the flag value override the one in config.
	if d.patchesDir != "" {
		return d.patchesDir
	}
	if d.cfg.Patches != nil {
		return d.cfg.Patches.Directory
	}
	return ""
}

func (d *initData) IkniteCluster() *v1alpha1.IkniteCluster {
	return d.ikniteCluster
}

func (d *initData) KubeletProcess() host.Process {
	return d.kubeletProcess
}

func (d *initData) SetKubeletProcess(process host.Process) {
	d.kubeletProcess = process
}

func (d *initData) SetMDnsConn(conn *mdns.Conn) {
	d.mdnsConn = conn
}

func (d *initData) CloseMDnsConn() error {
	if d.mdnsConn != nil {
		err := closeMDNSConn(d.mdnsConn)
		if err != nil {
			return fmt.Errorf("failed to close mdns connection: %w", err)
		}
		d.mdnsConn = nil
	}
	return nil
}

func (d *initData) SetStatusServer(srv *ikniteServer.IkniteServer) {
	d.statusServer = srv
}

func (d *initData) StatusServer() *ikniteServer.IkniteServer {
	return d.statusServer
}

func (d *initData) Context() context.Context {
	return d.ctx
}

func (d *initData) KustomizeOptions() *utils.KustomizeOptions {
	return d.kustomizeOptions
}

func (d *initData) Host() host.Host {
	return d.alpineHost
}

func (d *initData) SetIkniteCluster(cluster *v1alpha1.IkniteCluster) {
	clusterCopy := *cluster
	d.ikniteCluster = &clusterCopy
	if d.statusServer != nil {
		d.statusServer.SetCluster(&clusterCopy)
	}
	d.ikniteCluster.Persist(d.Host())
}

func (d *initData) UpdateIkniteCluster(
	state ikniteApi.ClusterState,
	phase string,
	ready, unready []*v1alpha1.WorkloadState,
) {
	d.ikniteCluster.Update(state, phase, ready, unready)
	if d.statusServer != nil {
		d.statusServer.SetCluster(d.ikniteCluster)
	}
	d.ikniteCluster.Persist(d.Host())
}

func (d *initData) StopStatusServer() error {
	if d.statusServer != nil {
		err := shutdownStatusServer(d.statusServer)
		if err != nil {
			return fmt.Errorf("failed to shutdown status server: %w", err)
		}
		d.statusServer = nil
	}
	return nil
}

func (d *initData) ErrGroup() *errgroup.Group {
	return &d.errGroup
}
