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

// cSpell: words pkiutil certutil

import (
"crypto/tls"
"crypto/x509"
"encoding/json"
"io"
"net"
"net/http"
"os"
"path/filepath"
"strconv"
"testing"
"time"

"github.com/stretchr/testify/require"
certutil "k8s.io/client-go/util/cert"
kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

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

func TestEnsureServerCertAndKey(t *testing.T) {
t.Parallel()
dir := t.TempDir()
createTestCA(t, dir)

ip := net.ParseIP("192.168.1.1")

// First call creates the cert
err := server.EnsureServerCertAndKey(dir, ip)
require.NoError(t, err)

certPath, keyPath := pkiutil.PathsForCertAndKey(dir, constants.IkniteServerCertName)
require.FileExists(t, certPath)
require.FileExists(t, keyPath)

// Second call is idempotent
err = server.EnsureServerCertAndKey(dir, ip)
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

ip := net.ParseIP("127.0.0.1")
require.NoError(t, server.EnsureServerCertAndKey(dir, ip))

srv, err := server.NewIkniteServer(dir, 0)
require.NoError(t, err)
require.NotNil(t, srv)
}

func TestStatusEndpoint(t *testing.T) {
t.Parallel()
dir := t.TempDir()
createTestCA(t, dir)

ip := net.ParseIP("127.0.0.1")
require.NoError(t, server.EnsureServerCertAndKey(dir, ip))
require.NoError(t, server.EnsureClientCertAndKey(dir))

statusContent := `{"kind":"IkniteCluster","apiVersion":"iknite.kaweezle.com/v1alpha1"}`
statusFile := filepath.Join(dir, "status.json")
require.NoError(t, os.WriteFile(statusFile, []byte(statusContent), 0o644))

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

// Start a TLS listener on a random port to test the handler
testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}
data, readErr := os.ReadFile(statusFile)
if readErr != nil {
http.Error(w, "error", http.StatusInternalServerError)
return
}
w.Header().Set("Content-Type", "application/json")
_, _ = w.Write(data)
})

l, err := tls.Listen("tcp", "127.0.0.1:0",
&tls.Config{
Certificates: []tls.Certificate{serverTLSCert},
ClientCAs:    caPool,
ClientAuth:   tls.RequireAndVerifyClientCert,
MinVersion:   tls.VersionTLS12,
})
require.NoError(t, err)
actualPort := l.Addr().(*net.TCPAddr).Port

srv := &http.Server{Handler: testHandler}
go func() { _ = srv.Serve(l) }()
defer func() { _ = server.ShutdownServer(srv) }()

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
