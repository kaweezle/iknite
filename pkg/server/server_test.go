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

package server_test

// cSpell: words pkiutil certutil clientcmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/tools/clientcmd"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/server"
)

// createTestCA creates a self-signed CA certificate and key in a temp dir.
func createTestCA(t *testing.T, dir string) {
	t.Helper()
	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "test-ca",
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	require.NoError(t, err)
	require.NoError(t, pkiutil.WriteCertAndKey(dir, "ca", caCert, caKey))
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
	dir := t.TempDir()
	createTestCA(t, dir)

	dnsNames := []string{"iknite.local"}
	ips := []net.IP{net.ParseIP("192.168.1.1")}

	// First call creates the cert
	err := server.EnsureServerCertAndKey(dir, dnsNames, ips)
	require.NoError(t, err)

	certPath, keyPath := pkiutil.PathsForCertAndKey(dir, constants.IkniteServerCertName)
	require.FileExists(t, certPath)
	require.FileExists(t, keyPath)

	// Second call is idempotent
	err = server.EnsureServerCertAndKey(dir, dnsNames, ips)
	require.NoError(t, err)
}

func TestEnsureClientCertAndKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createTestCA(t, dir)

	// First call creates the cert
	err := server.EnsureClientCertAndKey(dir)
	require.NoError(t, err)

	certPath, keyPath := pkiutil.PathsForCertAndKey(dir, constants.IkniteClientCertName)
	require.FileExists(t, certPath)
	require.FileExists(t, keyPath)

	// Second call is idempotent
	err = server.EnsureClientCertAndKey(dir)
	require.NoError(t, err)
}

func TestNewIkniteServer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createTestCA(t, dir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(dir, []string{"iknite.local"}, []net.IP{spec.Ip}))

	srv, err := server.NewIkniteServer(dir, spec)
	require.NoError(t, err)
	require.NotNil(t, srv)
}

func TestSetClusterUpdatesInMemoryStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createTestCA(t, dir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(dir, []string{"iknite.local"}, []net.IP{spec.Ip}))

	iSrv, err := server.NewIkniteServer(dir, spec)
	require.NoError(t, err)

	cluster := &v1alpha1.IkniteCluster{}
	cluster.Kind = "IkniteCluster"
	cluster.APIVersion = "iknite.kaweezle.com/v1alpha1"
	iSrv.SetCluster(cluster)

	// Verify that SetCluster updated the in-memory state by routing a request
	// through ServeHTTP (no network needed – httptest.ResponseRecorder captures it).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/status", nil)
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

func TestStatusEndpoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createTestCA(t, dir)

	spec := makeTestSpec(0)
	require.NoError(t, server.EnsureServerCertAndKey(dir, []string{spec.DomainName}, []net.IP{spec.Ip}))
	require.NoError(t, server.EnsureClientCertAndKey(dir))

	// Build server TLS config
	caCertPEM, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	require.NoError(t, err)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caCertPEM))

	serverTLSCert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, constants.IkniteServerCertName+".crt"),
		filepath.Join(dir, constants.IkniteServerCertName+".key"),
	)
	require.NoError(t, err)

	// Prepare an IkniteCluster for the in-memory status
	cluster := &v1alpha1.IkniteCluster{}
	cluster.Kind = "IkniteCluster"
	cluster.APIVersion = "iknite.kaweezle.com/v1alpha1"

	// Create the IkniteServer and seed it with the cluster status
	iSrv, err := server.NewIkniteServer(dir, spec)
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
	clientTLSCert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, constants.IkniteClientCertName+".crt"),
		filepath.Join(dir, constants.IkniteClientCertName+".key"),
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
	dir := t.TempDir()
	createTestCA(t, dir)

	spec := makeTestSpec(11443)
	require.NoError(t, server.EnsureServerCertAndKey(dir, []string{spec.DomainName}, []net.IP{spec.Ip}))
	require.NoError(t, server.EnsureClientCertAndKey(dir))

	confPath := filepath.Join(dir, "iknite.conf")
	err := server.EnsureIkniteConf(dir, confPath, spec)
	require.NoError(t, err)
	require.FileExists(t, confPath)

	// Load the kubeconfig and validate its content
	kubeConfig, err := clientcmd.LoadFromFile(confPath)
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
	err = server.EnsureIkniteConf(dir, confPath, spec)
	require.NoError(t, err)
}
