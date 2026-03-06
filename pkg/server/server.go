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

package server

// cSpell: words pkiutil certutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeConfigUtil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
)

// IkniteServer encapsulates the HTTPS status server together with the
// configuration and an in-memory snapshot of the cluster status.
// Access to the cluster snapshot is protected by a read/write mutex so that
// concurrent HTTP requests and background status updates do not race.
type IkniteServer struct {
	httpServer  *http.Server
	spec        *v1alpha1.IkniteClusterSpec
	mu          sync.RWMutex
	clusterJSON []byte // pre-serialized cluster, updated by SetCluster
}

// SetCluster serializes c to JSON and stores it under the write lock so that
// subsequent /status requests serve the latest in-memory state.
func (s *IkniteServer) SetCluster(c *v1alpha1.IkniteCluster) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to serialize cluster status for in-memory cache")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterJSON = data
}

// statusHandler serves the current cluster status as JSON.
func (s *IkniteServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	data := s.clusterJSON
	s.mu.RUnlock()

	if data == nil {
		http.Error(w, "status not available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(data); err != nil {
		log.WithError(err).Error("Failed to write status response")
	}
}

// ServeHTTP implements http.Handler so that IkniteServer can be used directly
// as a handler for a custom http.Server (useful in tests or when embedding
// the mux into a larger application).
func (s *IkniteServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer.Handler.ServeHTTP(w, r)
}

// Shutdown gracefully shuts down the server with a 10-second timeout.
func (s *IkniteServer) Shutdown() error {
	if s == nil {
		return nil
	}
	log.Info("Shutting down iknite status server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown iknite status server: %w", err)
	}
	return nil
}

// EnsureServerCertAndKey ensures that the iknite server certificate and key
// exist in certDir. If they don't exist, they are created signed by the
// Kubernetes CA. dnsNames and ips extend the built-in SANs (iknite, localhost,
// 127.0.0.1) with values from the cluster configuration.
func EnsureServerCertAndKey(certDir string, dnsNames []string, ips []net.IP) error {
	if pkiutil.CertOrKeyExist(certDir, constants.IkniteServerCertName) {
		log.WithField("certDir", certDir).Debug("Server cert already exists, skipping creation")
		return nil
	}

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(certDir, "ca")
	if err != nil {
		return fmt.Errorf("failed to load CA cert and key: %w", err)
	}

	altNames := certutil.AltNames{
		DNSNames: append([]string{"iknite", "localhost"}, dnsNames...),
		IPs:      append([]net.IP{net.ParseIP("127.0.0.1")}, ips...),
	}

	certConfig := &pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   "iknite-server",
			Organization: []string{"system:iknite"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, certConfig)
	if err != nil {
		return fmt.Errorf("failed to create server cert and key: %w", err)
	}

	if err := pkiutil.WriteCertAndKey(certDir, constants.IkniteServerCertName, cert, key); err != nil {
		return fmt.Errorf("failed to write server cert and key: %w", err)
	}

	log.WithField("certDir", certDir).Info("Server cert and key created")
	return nil
}

// EnsureClientCertAndKey ensures that the iknite client certificate and key
// exist in certDir. If they don't exist, they are created signed by the
// Kubernetes CA.
func EnsureClientCertAndKey(certDir string) error {
	if pkiutil.CertOrKeyExist(certDir, constants.IkniteClientCertName) {
		log.WithField("certDir", certDir).Debug("Client cert already exists, skipping creation")
		return nil
	}

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(certDir, "ca")
	if err != nil {
		return fmt.Errorf("failed to load CA cert and key: %w", err)
	}

	certConfig := &pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   "iknite-client",
			Organization: []string{"system:iknite"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	}

	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, certConfig)
	if err != nil {
		return fmt.Errorf("failed to create client cert and key: %w", err)
	}

	if err := pkiutil.WriteCertAndKey(certDir, constants.IkniteClientCertName, cert, key); err != nil {
		return fmt.Errorf("failed to write client cert and key: %w", err)
	}

	log.WithField("certDir", certDir).Info("Client cert and key created")
	return nil
}

// EnsureIkniteConf generates a kubeconfig-style client configuration file at
// confPath with the CA cert and client cert/key embedded. The server URL is
// derived from spec.Ip (or localhost as fallback) and spec.StatusServerPort.
// This file is analogous to /etc/kubernetes/admin.conf but targets the iknite
// status server. It is (re-)created on every start so that the IP address is
// always up to date.
func EnsureIkniteConf(certDir, confPath string, spec *v1alpha1.IkniteClusterSpec) error {
	// Determine the server address from the cluster spec
	serverAddr := "localhost"
	if spec.Ip != nil && !spec.Ip.IsLoopback() {
		serverAddr = spec.Ip.String()
	} else if spec.DomainName != "" {
		serverAddr = spec.DomainName
	}
	serverURL := fmt.Sprintf("https://%s", net.JoinHostPort(serverAddr, strconv.Itoa(spec.StatusServerPort)))

	caCertPEM, err := os.ReadFile(filepath.Join(certDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}

	clientCertPEM, err := os.ReadFile(filepath.Join(certDir, constants.IkniteClientCertName+".crt"))
	if err != nil {
		return fmt.Errorf("failed to read client cert: %w", err)
	}

	clientKeyPEM, err := os.ReadFile(filepath.Join(certDir, constants.IkniteClientCertName+".key"))
	if err != nil {
		return fmt.Errorf("failed to read client key: %w", err)
	}

	kubeconfig := kubeConfigUtil.CreateWithCerts(
		serverURL,
		constants.IkniteConfName, // cluster name
		constants.IkniteConfName, // user name
		caCertPEM,
		clientKeyPEM,
		clientCertPEM,
	)
	// Note: CreateWithCerts (via CreateBasic) already sets CurrentContext to
	// "<user>@<cluster>" = "iknite@iknite". Do not overwrite it.

	if err := kubeConfigUtil.WriteToDisk(confPath, kubeconfig); err != nil {
		return fmt.Errorf("failed to write iknite.conf to %s: %w", confPath, err)
	}

	log.WithFields(log.Fields{
		"path":   confPath,
		"server": serverURL,
	}).Info("iknite.conf written")
	return nil
}

// NewIkniteServer builds an IkniteServer from spec and the certificates in
// certDir. The port is taken from spec.StatusServerPort.
func NewIkniteServer(certDir string, spec *v1alpha1.IkniteClusterSpec) (*IkniteServer, error) {
	caCertPEM, err := os.ReadFile(filepath.Join(certDir, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	tlsCert, err := tls.LoadX509KeyPair(
		filepath.Join(certDir, constants.IkniteServerCertName+".crt"),
		filepath.Join(certDir, constants.IkniteServerCertName+".key"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert and key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	s := &IkniteServer{spec: spec}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.statusHandler)

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(spec.StatusServerPort))
	s.httpServer = &http.Server{
		Addr:      addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return s, nil
}

// StartIkniteServer creates certificates (if needed), builds, and starts the
// iknite HTTPS server. The initial cluster status is set from cluster.
// The returned IkniteServer's SetCluster method should be called on every
// subsequent cluster status update.
func StartIkniteServer(certDir string, spec *v1alpha1.IkniteClusterSpec, cluster *v1alpha1.IkniteCluster) (*IkniteServer, error) {
	// Build SAN extensions from the cluster spec.
	var dnsNames []string
	if spec.DomainName != "" {
		dnsNames = append(dnsNames, spec.DomainName)
	}
	var ips []net.IP
	if spec.Ip != nil && !spec.Ip.IsLoopback() {
		ips = append(ips, spec.Ip)
	}

	if err := EnsureServerCertAndKey(certDir, dnsNames, ips); err != nil {
		return nil, fmt.Errorf("failed to ensure server cert: %w", err)
	}
	if err := EnsureClientCertAndKey(certDir); err != nil {
		return nil, fmt.Errorf("failed to ensure client cert: %w", err)
	}
	if err := EnsureIkniteConf(certDir, constants.IkniteConfPath, spec); err != nil {
		return nil, fmt.Errorf("failed to write iknite client config to %s: %w", constants.IkniteConfPath, err)
	}

	srv, err := NewIkniteServer(certDir, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create iknite server: %w", err)
	}

	// Seed the in-memory status with the current cluster state.
	if cluster != nil {
		srv.SetCluster(cluster)
	}

	startErr := make(chan error, 1)
	go func() {
		log.WithFields(log.Fields{
			"addr": srv.httpServer.Addr,
		}).Info("Starting iknite status server")
		if listenErr := srv.httpServer.ListenAndServeTLS("", ""); listenErr != nil && listenErr != http.ErrServerClosed {
			log.WithError(listenErr).Error("Iknite status server error")
			startErr <- listenErr
		}
	}()

	// Give the server a brief moment to start and detect immediate failures
	// (e.g., port already in use).
	select {
	case err = <-startErr:
		return nil, fmt.Errorf("iknite status server failed to start: %w", err)
	case <-time.After(50 * time.Millisecond):
		// Server started without an immediate error.
	}

	return srv, nil
}

// ShutdownServer is a convenience wrapper around (*IkniteServer).Shutdown.
func ShutdownServer(srv *IkniteServer) error {
	return srv.Shutdown()
}
