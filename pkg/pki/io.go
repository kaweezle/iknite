// cSpell: words keyutil pkiutil wrapcheck
//
//nolint:wrapcheck // Want to preserve the error type for os.IsNotExist checks
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/host"
)

// PathsForCertAndKey returns the paths for the certificate and key given the path and basename.
func PathsForCertAndKey(pkiPath, name string) (string, string) {
	return pathForCert(pkiPath, name), pathForKey(pkiPath, name)
}

func pathForCert(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

func pathForKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

// WriteCertAndKey stores certificate and key at the specified location.
func WriteCertAndKey(
	fs host.FileSystem,
	pkiPath,
	name string,
	certificate *x509.Certificate,
	key crypto.Signer,
) error {
	if err := WriteKey(fs, pkiPath, name, key); err != nil {
		return fmt.Errorf("couldn't write key: %w", err)
	}

	return WriteCert(fs, pkiPath, name, certificate)
}

func writeKey(fs host.FileSystem, keyPath string, data []byte) error {
	if err := fs.MkdirAll(filepath.Dir(keyPath), os.FileMode(0o755)); err != nil {
		return err
	}
	return fs.WriteFile(keyPath, data, os.FileMode(0o600))
}

func writeCert(fs host.FileSystem, certPath string, data []byte) error {
	if err := fs.MkdirAll(filepath.Dir(certPath), os.FileMode(0o755)); err != nil {
		return err
	}
	return fs.WriteFile(certPath, data, os.FileMode(0o644))
}

// WriteKey stores the given key at the given location.
func WriteKey(fs host.FileSystem, pkiPath, name string, key crypto.Signer) error {
	if key == nil {
		return errors.New("private key cannot be nil when writing to file")
	}

	privateKeyPath := pathForKey(pkiPath, name)
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return fmt.Errorf("unable to marshal private key to PEM: %w", err)
	}
	if err := writeKey(fs, privateKeyPath, encoded); err != nil {
		return fmt.Errorf("unable to write private key to file %s: %w", privateKeyPath, err)
	}

	return nil
}

// WriteCert stores the given certificate at the given location.
func WriteCert(fs host.FileSystem, pkiPath, name string, certificate *x509.Certificate) error {
	if certificate == nil {
		return errors.New("certificate cannot be nil when writing to file")
	}

	certificatePath := pathForCert(pkiPath, name)
	if err := writeCert(fs, certificatePath, pkiutil.EncodeCertPEM(certificate)); err != nil {
		return fmt.Errorf("unable to write certificate to file %s: %w", certificatePath, err)
	}

	return nil
}

// CertOrKeyExist returns a boolean whether the cert or the key exists.
func CertOrKeyExist(fs host.FileSystem, pkiPath, name string) bool {
	certificatePath, privateKeyPath := PathsForCertAndKey(pkiPath, name)

	_, certErr := fs.Stat(certificatePath)
	_, keyErr := fs.Stat(privateKeyPath)

	return !os.IsNotExist(certErr) || !os.IsNotExist(keyErr)
}

// TryLoadCertAndKeyFromDisk tries to load a cert and a key from the disk and validates that they are valid.
func TryLoadCertAndKeyFromDisk(fs host.FileSystem, pkiPath, name string) (*x509.Certificate, crypto.Signer, error) {
	certificate, err := TryLoadCertFromDisk(fs, pkiPath, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	key, err := TryLoadKeyFromDisk(fs, pkiPath, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load key: %w", err)
	}

	return certificate, key, nil
}

// TryLoadCertFromDisk tries to load the cert from the disk.
func TryLoadCertFromDisk(fs host.FileSystem, pkiPath, name string) (*x509.Certificate, error) {
	certificatePath := pathForCert(pkiPath, name)

	certs, err := CertsFromFile(fs, certificatePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't load the certificate file %s: %w", certificatePath, err)
	}

	// Safely pick the first one because the sender's certificate must come first in the list.
	// For details, see: https://www.rfc-editor.org/rfc/rfc4346#section-7.4.2
	certificate := certs[0]

	return certificate, nil
}

// TryLoadKeyFromDisk tries to load the key from the disk and validates that it is valid.
func TryLoadKeyFromDisk(fs host.FileSystem, pkiPath, name string) (crypto.Signer, error) {
	privateKeyPath := pathForKey(pkiPath, name)

	// Parse the private key from a file
	privKey, err := PrivateKeyFromFile(fs, privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't load the private key file %s: %w", privateKeyPath, err)
	}

	// Allow RSA and ECDSA formats only
	var key crypto.Signer
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		key = k
	case *ecdsa.PrivateKey:
		key = k
	default:
		return nil, fmt.Errorf("the private key file %s is neither in RSA nor ECDSA format", privateKeyPath)
	}

	return key, nil
}

// PrivateKeyFromFile returns the private key in rsa.PrivateKey or ecdsa.PrivateKey format from a given PEM-encoded
// file.
// Returns an error if the file could not be read or if the private key could not be parsed.
func PrivateKeyFromFile(fs host.FileSystem, file string) (interface{}, error) {
	data, err := fs.ReadFile(file)
	if err != nil {
		return nil, err
	}
	key, err := keyutil.ParsePrivateKeyPEM(data)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file %s: %w", file, err)
	}
	return key, nil
}

func CertsFromFile(fs host.FileSystem, file string) ([]*x509.Certificate, error) {
	pemBlock, err := fs.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("couldn't read the certificate file %s: %w", file, err)
	}
	certs, err := cert.ParseCertsPEM(pemBlock)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the certificate file %s: %w", file, err)
	}
	return certs, nil
}

//nolint:wrapcheck // Want to preserve the error type for os.IsNotExist checks
func LoadX509KeyPair(fs host.FileSystem, certFile, keyFile string) (tls.Certificate, error) {
	certPEMBlock, err := fs.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEMBlock, err := fs.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}
