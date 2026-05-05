// cSpell: words apimachinery bootstraptoken bootstraptokenv1 certutil clientcmdapi
// cSpell: words clientsetfake genericclioptions paralleltest pkiutil
//
//nolint:paralleltest // Tests use shared kubeadm paths and multicast sockets
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pion/mdns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	kubeadmApi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeconfigPhase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	mockGenericCLI "github.com/kaweezle/iknite/mocks/k8s.io/cli-runtime/pkg/genericclioptions"
	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
	ikniteServer "github.com/kaweezle/iknite/pkg/server"
	"github.com/kaweezle/iknite/pkg/utils"
)

const testPKIDir = "pki"

type testContextKey string

func createTestBootstrapToken(t *testing.T, token string) bootstraptokenv1.BootstrapToken {
	t.Helper()

	tokenString, err := bootstraptokenv1.NewBootstrapTokenString(token)
	require.NoError(t, err)

	return bootstraptokenv1.BootstrapToken{Token: tokenString}
}

func createTestStatusServer(t *testing.T) *ikniteServer.IkniteServer {
	t.Helper()

	fs := host.NewMemMapFS()
	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "test-ca",
		},
		EncryptionAlgorithm: kubeadmApi.EncryptionAlgorithmRSA2048,
	})
	require.NoError(t, err)
	require.NoError(t, pki.WriteCertAndKey(fs, testPKIDir, "ca", caCert, caKey))

	spec := &v1alpha1.IkniteClusterSpec{
		DomainName:       "iknite.local",
		Ip:               net.ParseIP("192.168.1.1"),
		StatusServerPort: 0,
	}
	require.NoError(
		t,
		ikniteServer.EnsureServerCertAndKey(fs, testPKIDir, []string{spec.DomainName}, []net.IP{spec.Ip}),
	)

	srv, err := ikniteServer.NewIkniteServer(fs, testPKIDir, spec)
	require.NoError(t, err)

	return srv
}

func createTestMDNSConn(t *testing.T) *mdns.Conn {
	t.Helper()

	addr, err := net.ResolveUDPAddr("udp", mdns.DefaultAddress)
	require.NoError(t, err)

	socket, err := net.ListenUDP("udp4", addr)
	require.NoError(t, err)

	conn, err := mdns.Server(ipv4.NewPacketConn(socket), &mdns.Config{
		LocalNames: []string{"iknite.local"},
	})
	require.NoError(t, err)

	return conn
}

func decodeStatusResponse(t *testing.T, srv *ikniteServer.IkniteServer) *v1alpha1.IkniteCluster {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/status", http.NoBody)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	cluster := &v1alpha1.IkniteCluster{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), cluster))

	return cluster
}

func TestInitDataAccessors(t *testing.T) {
	req := require.New(t)
	output := &bytes.Buffer{}
	kustomizeOptions := &utils.KustomizeOptions{Kustomization: "/kustomization", ForceConfig: true}
	ctx := context.WithValue(context.Background(), testContextKey("key"), "value")
	process := mockHost.NewMockProcess(t)

	data := &initData{
		cfg: &kubeadmApi.InitConfiguration{
			CertificateKey: "initial-key",
			BootstrapTokens: []bootstraptokenv1.BootstrapToken{
				createTestBootstrapToken(t, "abcdef.0123456789abcdef"),
			},
			Patches: &kubeadmApi.Patches{Directory: "/config-patches"},
		},
		skipTokenPrint:          true,
		certificatesDir:         "/etc/kubernetes/pki",
		externalCA:              true,
		outputWriter:            output,
		uploadCerts:             true,
		skipCertificateKeyPrint: true,
		patchesDir:              "/flag-patches",
		ctx:                     ctx,
		kustomizeOptions:        kustomizeOptions,
	}

	req.True(data.UploadCerts())
	req.Equal("initial-key", data.CertificateKey())
	data.SetCertificateKey("updated-key")
	req.Equal("updated-key", data.CertificateKey())
	req.True(data.SkipCertificateKeyPrint())
	req.True(data.SkipTokenPrint())
	req.Equal("/etc/kubernetes/pki", data.CertificateDir())
	req.True(data.ExternalCA())
	req.Same(output, data.OutputWriter())
	req.Equal([]string{"abcdef.0123456789abcdef"}, data.Tokens())
	req.Equal("/flag-patches", data.PatchesDir())

	data.patchesDir = ""
	req.Equal("/config-patches", data.PatchesDir())
	data.cfg.Patches = nil
	req.Empty(data.PatchesDir())

	req.Nil(data.KubeletProcess())
	data.SetKubeletProcess(process)
	req.Same(process, data.KubeletProcess())
	req.Equal(ctx, data.Context())
	req.Same(kustomizeOptions, data.KustomizeOptions())

	errGroup := data.ErrGroup()
	errGroup.Go(func() error { return nil })
	req.NoError(errGroup.Wait())
}

func TestInitDataKubeConfigAndRESTClientGetter(t *testing.T) {
	req := require.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}))
	defer server.Close()

	kubeconfigPath := filepath.Join(t.TempDir(), "custom.conf")
	err := host.NewDefaultHost().WriteFile(
		kubeconfigPath,
		fmt.Appendf(nil, testKubeconfigDataFormat, server.URL),
		0o600,
	)
	req.NoError(err)

	data := &initData{
		cfg: &kubeadmApi.InitConfiguration{
			LocalAPIEndpoint: kubeadmApi.APIEndpoint{
				AdvertiseAddress: "127.0.0.1",
				BindPort:         6443,
			},
		},
		kubeconfigPath: kubeconfigPath,
		alpineHost:     host.NewDefaultHost(),
	}

	kubeconfig, err := data.KubeConfig()
	req.NoError(err)
	cachedKubeconfig, err := data.KubeConfig()
	req.NoError(err)
	req.Same(kubeconfig, cachedKubeconfig)

	getter, err := data.RESTClientGetter()
	req.NoError(err)
	cachedGetter, err := data.RESTClientGetter()
	req.NoError(err)
	req.Same(getter, cachedGetter)

	_, err = data.WaitControlPlaneClient()
	req.NoError(err)
	req.Equal(server.URL, kubeconfig.Clusters["foo-cluster"].Server)

	invalidWaitClientData := &initData{
		cfg: &kubeadmApi.InitConfiguration{
			LocalAPIEndpoint: kubeadmApi.APIEndpoint{
				AdvertiseAddress: "bad host",
				BindPort:         6443,
			},
		},
		kubeconfigPath: kubeconfigPath,
		alpineHost:     host.NewDefaultHost(),
	}
	_, err = invalidWaitClientData.WaitControlPlaneClient()
	req.ErrorContains(err, "failed to create client set from config")

	client, err := data.ClientWithoutBootstrap()
	req.NoError(err)
	req.NoError(client.Discovery().RESTClient().Verb(http.MethodHead).Do(context.Background()).Error())

	missingPath := filepath.Join(t.TempDir(), "missing.conf")
	errorData := &initData{
		cfg:            data.cfg,
		kubeconfigPath: missingPath,
		alpineHost:     host.NewDefaultHost(),
	}

	_, err = errorData.KubeConfig()
	req.ErrorContains(err, "failed to load kubeconfig from file")
	_, err = errorData.RESTClientGetter()
	req.ErrorContains(err, "failed to get kubeconfig for REST client getter")
	_, err = errorData.WaitControlPlaneClient()
	req.ErrorContains(err, "failed to load kubeconfig for wait control plane")
	_, err = errorData.ClientWithoutBootstrap()
	req.Error(err)

	getterMock := mockGenericCLI.NewMockRESTClientGetter(t)
	cachedData := &initData{clientGetter: getterMock}
	cachedClientGetter, err := cachedData.RESTClientGetter()
	req.NoError(err)
	req.Same(getterMock, cachedClientGetter)
}

func TestInitDataClientBranches(t *testing.T) {
	req := require.New(t)
	originalEnsureAdminClusterRoleBinding := ensureAdminClusterRoleBinding
	t.Cleanup(func() {
		ensureAdminClusterRoleBinding = originalEnsureAdminClusterRoleBinding
	})

	cachedClient := clientsetfake.NewSimpleClientset()
	data := &initData{client: cachedClient}
	client, err := data.Client()
	req.NoError(err)
	req.Same(cachedClient, client)

	ensureAdminClusterRoleBinding = func(string, kubeconfigPhase.EnsureRBACFunc) (clientset.Interface, error) {
		return cachedClient, nil
	}
	bootstrapSuccessData := &initData{
		cfg:            &kubeadmApi.InitConfiguration{},
		kubeconfigPath: kubeadmConstants.GetAdminKubeConfigPath(),
	}
	client, err = bootstrapSuccessData.Client()
	req.NoError(err)
	req.Same(cachedClient, client)
	req.True(bootstrapSuccessData.adminKubeConfigBootstrapped)

	ensureAdminClusterRoleBinding = originalEnsureAdminClusterRoleBinding

	bootstrapErrorData := &initData{
		cfg:            &kubeadmApi.InitConfiguration{},
		kubeconfigPath: kubeadmConstants.GetAdminKubeConfigPath(),
	}
	_, err = bootstrapErrorData.Client()
	req.ErrorContains(err, "could not bootstrap the admin user in file admin.conf")

	customPathErrorData := &initData{
		cfg:            &kubeadmApi.InitConfiguration{},
		kubeconfigPath: filepath.Join(t.TempDir(), "missing.conf"),
		alpineHost:     host.NewDefaultHost(),
	}
	_, err = customPathErrorData.Client()
	req.ErrorContains(err, "failed to get REST client getter")

	getterMock := mockGenericCLI.NewMockRESTClientGetter(t)
	getterMock.EXPECT().ToRESTConfig().Return((*rest.Config)(nil), errors.New("bad rest config")).Once()
	invalidClientData := &initData{
		cfg:            &kubeadmApi.InitConfiguration{},
		kubeconfigPath: filepath.Join(t.TempDir(), "custom.conf"),
		clientGetter:   getterMock,
	}
	_, err = invalidClientData.Client()
	req.ErrorContains(err, "failed to create client set from file")

	opts := newInitOptions()
	initRunner := workflow.NewRunner()
	var output bytes.Buffer
	cmd := newCmdInit(&output, opts, initRunner, host.NewDefaultHost())
	req.NoError(cmd.Flags().Set(options.DryRun, "true"))
	t.Setenv("KUBEADM_INIT_DRYRUN_DIR", filepath.Join(t.TempDir(), "dry-run"))

	runData, err := initRunner.InitData(nil)
	req.NoError(err)

	dryRunData, ok := runData.(*initData)
	req.True(ok)

	err = dryRunData.Host().WriteFile(
		dryRunData.KubeConfigPath(),
		fmt.Appendf(nil, testKubeconfigDataFormat, "https://127.0.0.1:6443"),
		0o600,
	)
	req.NoError(err)

	client, err = dryRunData.Client()
	req.NoError(err)
	req.NotNil(client)

	client, err = dryRunData.ClientWithoutBootstrap()
	req.NoError(err)
	req.NotNil(client)
}

func TestInitDataStatusServerAndMDNS(t *testing.T) {
	req := require.New(t)
	originalCloseMDNSConn := closeMDNSConn
	originalShutdownStatusServer := shutdownStatusServer
	t.Cleanup(func() {
		closeMDNSConn = originalCloseMDNSConn
		shutdownStatusServer = originalShutdownStatusServer
	})

	alpineHost := mockHost.NewMockHost(t)
	alpineHost.EXPECT().MkdirAll(constants.StatusDirectory, os.FileMode(0o755)).Return(nil).Twice()
	alpineHost.EXPECT().WriteFile(constants.StatusFile, mock.Anything, os.FileMode(0o644)).Return(nil).Twice()

	data := &initData{
		cfg:           &kubeadmApi.InitConfiguration{},
		ikniteCluster: &v1alpha1.IkniteCluster{},
		alpineHost:    alpineHost,
	}

	req.Nil(data.StatusServer())
	statusServer := createTestStatusServer(t)
	data.SetStatusServer(statusServer)
	req.Same(statusServer, data.StatusServer())

	cluster := &v1alpha1.IkniteCluster{
		TypeMeta: metaV1.TypeMeta{
			Kind:       ikniteApi.IkniteClusterKind,
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		Spec: v1alpha1.IkniteClusterSpec{
			ClusterName: "initial-cluster",
		},
	}
	data.SetIkniteCluster(cluster)
	cluster.Spec.ClusterName = "mutated-cluster"
	req.Equal("initial-cluster", data.IkniteCluster().Spec.ClusterName)
	req.Equal("initial-cluster", decodeStatusResponse(t, statusServer).Spec.ClusterName)

	ready := []*v1alpha1.WorkloadState{{Namespace: "default", Name: "ready", Ok: true}}
	unready := []*v1alpha1.WorkloadState{{Namespace: "default", Name: "unready", Ok: false}}
	data.UpdateIkniteCluster(ikniteApi.Stabilizing, "workloads", ready, unready)
	req.Equal(ikniteApi.Stabilizing, data.IkniteCluster().Status.State)
	req.Equal("workloads", data.IkniteCluster().Status.CurrentPhase)
	req.Equal(2, data.IkniteCluster().Status.WorkloadsState.Count)

	status := decodeStatusResponse(t, statusServer)
	req.Equal("workloads", status.Status.CurrentPhase)
	req.Equal(1, status.Status.WorkloadsState.ReadyCount)
	req.Equal(1, status.Status.WorkloadsState.UnreadyCount)

	req.NoError(data.StopStatusServer())
	req.Nil(data.StatusServer())
	req.NoError(data.StopStatusServer())

	shutdownStatusServer = func(*ikniteServer.IkniteServer) error {
		return errors.New("shutdown failed")
	}
	data.SetStatusServer(&ikniteServer.IkniteServer{})
	req.ErrorContains(data.StopStatusServer(), "failed to shutdown status server")
	shutdownStatusServer = originalShutdownStatusServer

	data.SetMDnsConn(createTestMDNSConn(t))
	req.NoError(data.CloseMDnsConn())
	req.NoError(data.CloseMDnsConn())

	closeMDNSConn = func(*mdns.Conn) error {
		return errors.New("close failed")
	}
	data.SetMDnsConn(&mdns.Conn{})
	req.ErrorContains(data.CloseMDnsConn(), "failed to close mdns connection")
}
