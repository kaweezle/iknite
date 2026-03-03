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
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/constants"
)

// EnsureServerCertAndKey ensures that the iknite server certificate and key
// exist in the given directory. If they don't exist, they are created and
// signed by the Kubernetes CA.
func EnsureServerCertAndKey(certDir string, ip net.IP) error {
	if pkiutil.CertOrKeyExist(certDir, constants.IkniteServerCertName) {
		log.WithField("certDir", certDir).Debug("Server cert already exists, skipping creation")
		return nil
	}

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(certDir, "ca")
	if err != nil {
		return fmt.Errorf("failed to load CA cert and key: %w", err)
	}

	altNames := certutil.AltNames{
		DNSNames: []string{"iknite", "iknite.local", "localhost"},
		IPs:      []net.IP{net.ParseIP("127.0.0.1")},
	}
	if ip != nil && !ip.IsLoopback() {
		altNames.IPs = append(altNames.IPs, ip)
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
// exist in the given directory. If they don't exist, they are created and
// signed by the Kubernetes CA.
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

// statusHandler handles GET /status requests and returns the contents of
// the iknite status file as JSON.
func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := os.ReadFile(constants.StatusFile)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "status not available", http.StatusServiceUnavailable)
			return
		}
		log.WithError(err).Error("Failed to read status file")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.WithError(err).Error("Failed to write status response")
	}
}

// NewIkniteServer creates a new HTTPS server for the iknite status API.
// It uses mTLS with the Kubernetes CA so that only clients presenting a
// certificate signed by the same CA are allowed.
func NewIkniteServer(certDir string, port int) (*http.Server, error) {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/status", statusHandler)

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	server := &http.Server{
		Addr:      addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return server, nil
}

// StartIkniteServer creates and starts the iknite HTTPS server in a goroutine.
// It returns the server instance so that it can be shut down gracefully.
func StartIkniteServer(certDir string, ip net.IP, port int) (*http.Server, error) {
	if err := EnsureServerCertAndKey(certDir, ip); err != nil {
		return nil, fmt.Errorf("failed to ensure server cert: %w", err)
	}
	if err := EnsureClientCertAndKey(certDir); err != nil {
		return nil, fmt.Errorf("failed to ensure client cert: %w", err)
	}

	server, err := NewIkniteServer(certDir, port)
	if err != nil {
		return nil, fmt.Errorf("failed to create iknite server: %w", err)
	}

	go func() {
		log.WithFields(log.Fields{
			"addr": server.Addr,
		}).Info("Starting iknite status server")
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("Iknite status server error")
		}
	}()

	return server, nil
}

// ShutdownServer gracefully shuts down the server.
func ShutdownServer(server *http.Server) error {
	if server == nil {
		return nil
	}
	log.Info("Shutting down iknite status server...")
	if err := server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("failed to shutdown iknite status server: %w", err)
	}
	return nil
}
