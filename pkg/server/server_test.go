/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

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
//nolint:gosec,errcheck,forcetypeassert // Unit testing
package server_test

// cSpell: words pkiutil certutil clientcmd kubeadmapi noctx ikniteapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/pki"
	"github.com/kaweezle/iknite/pkg/server"
)

const pkiDir = "pki"

// createTestCA creates a self-signed CA certificate and key in a temp dir.
//
//nolint:unparam // Want to reuse the same CA for multiple tests
func createTestCA(t *testing.T, fs host.FileSystem, dir string) {
	t.Helper()
	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "test-ca",
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	require.NoError(t, err)
	require.NoError(t, pki.WriteCertAndKey(fs, dir, "ca", caCert, caKey))
}

func makeTestSpec(port int) *v1alpha1.IkniteClusterSpec {
	return &v1alpha1.IkniteClusterSpec{
		DomainName:       "iknite.local",
		Ip:               net.ParseIP("192.168.1.1"),
		StatusServerPort: port,
	}
}

func TestEnsureServerCertAndKey(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	dnsNames := []string{"iknite.local"}
	ips := []net.IP{net.ParseIP("192.168.1.1")}

	// First call creates the cert
	err := server.EnsureServerCertAndKey(fs, pkiDir, dnsNames, ips)
	require.NoError(t, err)

	certPath, keyPath := pki.PathsForCertAndKey(pkiDir, constants.IkniteServerCertName)
	ok, err := fs.Exists(certPath)
	require.NoError(t, err)
	require.True(t, ok, "expected cert file to exist at %s", certPath)
	ok, err = fs.Exists(keyPath)
	require.NoError(t, err)
	require.True(t, ok, "expected key file to exist at %s", keyPath)

	// Second call is idempotent
	err = server.EnsureServerCertAndKey(fs, pkiDir, dnsNames, ips)
	require.NoError(t, err)
}

func TestEnsureClientCertAndKey(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	// First call creates the cert
	err := server.EnsureClientCertAndKey(fs, pkiDir)
	require.NoError(t, err)

	certPath, keyPath := pkiutil.PathsForCertAndKey(pkiDir, constants.IkniteClientCertName)
	ok, err := fs.Exists(certPath)
	require.NoError(t, err)
	require.True(t, ok, "expected cert file to exist at %s", certPath)
	ok, err = fs.Exists(keyPath)
	require.NoError(t, err)
	require.True(t, ok, "expected key file to exist at %s", keyPath)

	// Second call is idempotent
	err = server.EnsureClientCertAndKey(fs, pkiDir)
	require.NoError(t, err)
}

func TestNewIkniteServer(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(fs, pkiDir, []string{"iknite.local"}, []net.IP{spec.Ip}))

	srv, err := server.NewIkniteServer(fs, pkiDir, spec)
	require.NoError(t, err)
	require.NotNil(t, srv)
}

func TestSetClusterUpdatesInMemoryStatus(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(fs, pkiDir, []string{"iknite.local"}, []net.IP{spec.Ip}))

	iSrv, err := server.NewIkniteServer(fs, pkiDir, spec)
	require.NoError(t, err)

	cluster := &v1alpha1.IkniteCluster{}
	cluster.Kind = "IkniteCluster"
	cluster.APIVersion = "iknite.kaweezle.com/v1alpha1"
	iSrv.SetCluster(cluster)

	// Verify that SetCluster updated the in-memory state by routing a request
	// through ServeHTTP (no network needed – httptest.ResponseRecorder captures it).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/status", http.NoBody)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	iSrv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.Bytes()
	require.True(t, json.Valid(body))

	var got v1alpha1.IkniteCluster
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "IkniteCluster", got.Kind)
	require.Equal(t, "iknite.kaweezle.com/v1alpha1", got.APIVersion)
}

func TestHealthzEndpoint(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(fs, pkiDir, []string{"iknite.local"}, []net.IP{spec.Ip}))

	iSrv, err := server.NewIkniteServer(fs, pkiDir, spec)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", http.NoBody)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	iSrv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	require.Equal(t, "ok", rec.Body.String())
}

func TestStatusEndpoint(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(fs, pkiDir, []string{spec.DomainName}, []net.IP{spec.Ip}))
	require.NoError(t, server.EnsureClientCertAndKey(fs, pkiDir))

	// Build server TLS config
	caCertPEM, err := fs.ReadFile(filepath.Join(pkiDir, "ca.crt"))
	require.NoError(t, err)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caCertPEM))

	_, _, err = pki.TryLoadCertAndKeyFromDisk(fs, pkiDir, constants.IkniteServerCertName)
	require.NoError(t, err)

	serverTLSCert, err := pki.LoadX509KeyPair(
		fs,
		filepath.Join(pkiDir, constants.IkniteServerCertName+".crt"),
		filepath.Join(pkiDir, constants.IkniteServerCertName+".key"),
	)
	require.NoError(t, err)

	// Prepare an IkniteCluster for the in-memory status
	cluster := &v1alpha1.IkniteCluster{}
	cluster.Kind = "IkniteCluster"
	cluster.APIVersion = "iknite.kaweezle.com/v1alpha1"

	// Create the IkniteServer and seed it with the cluster status
	iSrv, err := server.NewIkniteServer(fs, pkiDir, spec)
	require.NoError(t, err)
	iSrv.SetCluster(cluster)

	// Start a TLS listener on a random port
	l, err := tls.Listen("tcp", "127.0.0.1:0",
		&tls.Config{
			Certificates: []tls.Certificate{serverTLSCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			MinVersion:   tls.VersionTLS12,
		})
	require.NoError(t, err)
	actualPort := l.Addr().(*net.TCPAddr).Port

	// Build a plain http.Server using the IkniteServer's mux (exposed via ServeHTTP).
	httpSrv := &http.Server{Handler: iSrv}
	go func() { _ = httpSrv.Serve(l) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()

	// Build client with client cert
	clientTLSCert, err := pki.LoadX509KeyPair(
		fs,
		filepath.Join(pkiDir, constants.IkniteClientCertName+".crt"),
		filepath.Join(pkiDir, constants.IkniteClientCertName+".key"),
	)
	require.NoError(t, err)

	tlsClientConfig := &tls.Config{
		Certificates: []tls.Certificate{clientTLSCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}
	httpClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsClientConfig},
		Timeout:   5 * time.Second,
	}

	// Wait a tiny bit for the server to be ready
	time.Sleep(50 * time.Millisecond)

	url := "https://127.0.0.1:" + strconv.Itoa(actualPort) + "/status"
	resp, err := httpClient.Get(url) //nolint:noctx // test code
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, json.Valid(body))
}

func TestEnsureIkniteConf(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	spec := makeTestSpec(11443)
	require.NoError(t, server.EnsureServerCertAndKey(fs, pkiDir, []string{spec.DomainName}, []net.IP{spec.Ip}))
	require.NoError(t, server.EnsureClientCertAndKey(fs, pkiDir))

	confPath := filepath.Join(pkiDir, "iknite.conf")
	err := server.EnsureIkniteConf(fs, pkiDir, confPath, spec)
	require.NoError(t, err)
	ok, err := fs.Exists(confPath)
	require.NoError(t, err)
	require.True(t, ok, "expected iknite.conf to exist at %s", confPath)

	// Load the kubeconfig and validate its content
	kubeConfig, err := k8s.LoadFromFile(fs, confPath)
	require.NoError(t, err)
	require.NotEmpty(t, kubeConfig.CurrentContext)

	ctx := kubeConfig.Contexts[kubeConfig.CurrentContext]
	require.NotNil(t, ctx, "context %q should exist in loaded kubeconfig", kubeConfig.CurrentContext)

	cluster := kubeConfig.Clusters[ctx.Cluster]
	require.NotNil(t, cluster)
	require.Contains(t, cluster.Server, "11443", "server URL should include the port")
	require.NotEmpty(t, cluster.CertificateAuthorityData)

	authInfo := kubeConfig.AuthInfos[ctx.AuthInfo]
	require.NotNil(t, authInfo)
	require.NotEmpty(t, authInfo.ClientCertificateData)
	require.NotEmpty(t, authInfo.ClientKeyData)

	// Regeneration should succeed (overwrites with updated IP)
	err = server.EnsureIkniteConf(fs, pkiDir, confPath, spec)
	require.NoError(t, err)
}

func TestStatusAndHealthzMethodHandling(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)
	spec := makeTestSpec(0)
	req.NoError(server.EnsureServerCertAndKey(fs, pkiDir, []string{spec.DomainName}, []net.IP{spec.Ip}))

	iSrv, err := server.NewIkniteServer(fs, pkiDir, spec)
	req.NoError(err)

	postStatusReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/status", http.NoBody)
	req.NoError(err)
	postStatusRec := httptest.NewRecorder()
	iSrv.ServeHTTP(postStatusRec, postStatusReq)
	req.Equal(http.StatusMethodNotAllowed, postStatusRec.Code)

	getStatusReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/status", http.NoBody)
	req.NoError(err)
	getStatusRec := httptest.NewRecorder()
	iSrv.ServeHTTP(getStatusRec, getStatusReq)
	req.Equal(http.StatusServiceUnavailable, getStatusRec.Code)

	postHealthReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/healthz", http.NoBody)
	req.NoError(err)
	postHealthRec := httptest.NewRecorder()
	iSrv.ServeHTTP(postHealthRec, postHealthReq)
	req.Equal(http.StatusMethodNotAllowed, postHealthRec.Code)
}

func TestShutdownServerNil(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	var srv *server.IkniteServer
	req.NoError(server.ShutdownServer(srv))
}

func TestEnsureIkniteConfAddressSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		spec          *v1alpha1.IkniteClusterSpec
		wantServerSub string
	}{
		{
			name: "loopback with domain uses domain",
			spec: &v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.1"),
				DomainName:       "iknite.local",
				StatusServerPort: 11443,
			},
			wantServerSub: "iknite.local:11443",
		},
		{
			name: "loopback without domain uses localhost",
			spec: &v1alpha1.IkniteClusterSpec{
				Ip:               net.ParseIP("127.0.0.1"),
				StatusServerPort: 11443,
			},
			wantServerSub: "localhost:11443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			fs := host.NewMemMapFS()
			createTestCA(t, fs, pkiDir)
			req.NoError(server.EnsureClientCertAndKey(fs, pkiDir))

			confPath := filepath.Join(pkiDir, constants.IkniteConfName+".conf")
			req.NoError(server.EnsureIkniteConf(fs, pkiDir, confPath, tt.spec))

			cfg, err := k8s.LoadFromFile(fs, confPath)
			req.NoError(err)
			ctx := cfg.Contexts[cfg.CurrentContext]
			req.NotNil(ctx)
			req.Contains(cfg.Clusters[ctx.Cluster].Server, tt.wantServerSub)
		})
	}
}

func TestEnsureIkniteServerConfiguration(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	req := require.New(t)
	createTestCA(t, fs, pkiDir)

	err := server.EnsureIkniteServerConfiguration(fs, pkiDir, makeTestSpec(11443))
	require.NoError(t, err)

	ok, err := fs.Exists(constants.IkniteConfPath)
	req.NoError(err)
	req.True(ok, "expected iknite.conf to exist at %s", constants.IkniteConfPath)

	cfg, err := k8s.LoadFromFile(fs, constants.IkniteConfPath)
	req.NoError(err)
	req.NotEmpty(cfg.CurrentContext)
	req.True(ok, "expected iknite.conf to exist at %s", constants.IkniteConfPath)

	ctx := cfg.Contexts[cfg.CurrentContext]
	req.NotNil(ctx, "context %q should exist in loaded kubeconfig", cfg.CurrentContext)
	req.Contains(cfg.Clusters[ctx.Cluster].Server, "11443", "server URL should include the port")

	certPath, keyPath := pkiutil.PathsForCertAndKey(pkiDir, constants.IkniteClientCertName)
	ok, err = fs.Exists(certPath)
	req.NoError(err)
	req.True(ok, "expected cert file to exist at %s", certPath)
	ok, err = fs.Exists(keyPath)
	req.NoError(err)
	req.True(ok, "expected key file to exist at %s", keyPath)
}

func TestStartIkniteServer(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	createTestCA(t, fs, pkiDir)

	cluster, err := v1alpha1.LoadIkniteClusterOrDefault(fs)
	require.NoError(t, err)

	srv, err := server.StartIkniteServer(fs, pkiDir, cluster)
	require.NoError(t, err)
	require.NotNil(t, srv)

	// Wait a tiny bit for the server to be ready
	time.Sleep(50 * time.Millisecond)

	// Check that the server is listening on the expected port
	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", "localhost:11443")
	require.NoError(t, err)
	require.NotNil(t, conn)
	conn.Close()

	kubeClient, err := k8s.NewClientFromFile(fs, constants.IkniteConfPath)
	require.NoError(t, err)
	restClient, err := k8s.RESTClient(kubeClient)
	require.NoError(t, err)

	req := restClient.Get().AbsPath("/healthz")
	body, err := req.DoRaw(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))

	req = restClient.Get().AbsPath("/status")
	body, err = req.DoRaw(context.Background())
	require.NoError(t, err)
	require.True(t, json.Valid(body))

	ikniteCluster := &v1alpha1.IkniteCluster{}
	err = json.Unmarshal(body, ikniteCluster)
	require.NoError(t, err)
	// The unmarshaling sets the timezone to local but we default to UTC.
	ikniteCluster.Status.LastUpdateTimeStamp = ikniteCluster.Status.LastUpdateTimeStamp.Rfc3339Copy()
	// Check that the returned status matches the in-memory cluster (which should be empty/default)
	require.Equal(t, cluster, ikniteCluster)

	err = server.ShutdownServer(srv)
	require.NoError(t, err)
}
