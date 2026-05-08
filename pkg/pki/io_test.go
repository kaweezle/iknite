// cSpell: words pkiutil keyutil kubeadmapi certutil wrapcheck mykey eckey certonly keyonly ECDSAP
// cSpell: words ed25519key unmarshalable
package pki_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
)

const pkiDir = "pki"

func newTestCACert(t *testing.T, fs host.FileSystem) {
	t.Helper()
	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName: "test-ca",
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	require.NoError(t, err)
	require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey))
}

func TestPathsForCertAndKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkiPath  string
		certName string
		wantCert string
		wantKey  string
	}{
		{
			name:     "standard pki directory",
			pkiPath:  "/etc/kubernetes/pki",
			certName: "apiserver",
			wantCert: "/etc/kubernetes/pki/apiserver.crt",
			wantKey:  "/etc/kubernetes/pki/apiserver.key",
		},
		{
			name:     "relative path",
			pkiPath:  "pki",
			certName: "ca",
			wantCert: "pki/ca.crt",
			wantKey:  "pki/ca.key",
		},
		{
			name:     "nested cert name",
			pkiPath:  "/run/iknite",
			certName: "iknite-server",
			wantCert: "/run/iknite/iknite-server.crt",
			wantKey:  "/run/iknite/iknite-server.key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			certPath, keyPath := pki.PathsForCertAndKey(tt.pkiPath, tt.certName)
			require.Equal(t, tt.wantCert, certPath)
			require.Equal(t, tt.wantKey, keyPath)
		})
	}
}

func TestWriteKey(t *testing.T) {
	t.Parallel()

	t.Run("nil key returns error", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		err := pki.WriteKey(fs, pkiDir, "test", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "private key cannot be nil")
	})

	t.Run("RSA key is written successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)

		err = pki.WriteKey(fs, pkiDir, "mykey", caKey)
		require.NoError(t, err)

		_, keyPath := pki.PathsForCertAndKey(pkiDir, "mykey")
		ok, err := fs.Exists(keyPath)
		require.NoError(t, err)
		require.True(t, ok, "expected key file at %s", keyPath)
	})

	t.Run("ECDSA key is written successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		err = pki.WriteKey(fs, pkiDir, "eckey", ecKey)
		require.NoError(t, err)

		_, keyPath := pki.PathsForCertAndKey(pkiDir, "eckey")
		ok, err := fs.Exists(keyPath)
		require.NoError(t, err)
		require.True(t, ok, "expected key file at %s", keyPath)
	})

	t.Run("unmarshalable key type returns error", func(t *testing.T) {
		t.Parallel()
		// ed25519 keys are not supported by keyutil.MarshalPrivateKeyToPEM.
		_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		fs := host.NewMemMapFS()
		err = pki.WriteKey(fs, pkiDir, "ed25519key", ed25519Key)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to marshal private key to PEM")
	})
}

func TestWriteCert(t *testing.T) {
	t.Parallel()

	t.Run("nil certificate returns error", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		err := pki.WriteCert(fs, pkiDir, "test", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "certificate cannot be nil")
	})

	t.Run("certificate is written and reloads correctly", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, _, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)

		require.NoError(t, pki.WriteCert(fs, pkiDir, "ca", caCert))

		// Verify round-trip: the written cert must be loadable and have the same serial.
		loaded, err := pki.TryLoadCertFromDisk(fs, pkiDir, "ca")
		require.NoError(t, err)
		require.Equal(t, caCert.SerialNumber, loaded.SerialNumber)
	})
}

func TestWriteCertAndKey(t *testing.T) {
	t.Parallel()

	t.Run("both cert and key are written", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)

		err = pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey)
		require.NoError(t, err)

		certPath, keyPath := pki.PathsForCertAndKey(pkiDir, "ca")
		ok, err := fs.Exists(certPath)
		require.NoError(t, err)
		require.True(t, ok, "expected cert file at %s", certPath)

		ok, err = fs.Exists(keyPath)
		require.NoError(t, err)
		require.True(t, ok, "expected key file at %s", keyPath)
	})
}

// TestWrite_MkdirAllError verifies that WriteKey, WriteCert and WriteCertAndKey all
// propagate a filesystem MkdirAll failure as a wrapped error.
func TestWrite_MkdirAllError(t *testing.T) {
	t.Parallel()

	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config:              certutil.Config{CommonName: "test-ca"},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		fn      func(host.FileSystem) error
		wantErr string
	}{
		{
			name:    "WriteKey propagates mkdir error",
			fn:      func(fs host.FileSystem) error { return pki.WriteKey(fs, pkiDir, "fail", caKey) },
			wantErr: "unable to write private key to file",
		},
		{
			name:    "WriteCert propagates mkdir error",
			fn:      func(fs host.FileSystem) error { return pki.WriteCert(fs, pkiDir, "fail", caCert) },
			wantErr: "unable to write certificate to file",
		},
		{
			name:    "WriteCertAndKey propagates WriteKey error",
			fn:      func(fs host.FileSystem) error { return pki.WriteCertAndKey(fs, pkiDir, "fail", caCert, caKey) },
			wantErr: "couldn't write key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockFS := mockHost.NewMockFileSystem(t)
			mockFS.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(fmt.Errorf("mkdir failed"))
			require.ErrorContains(t, tt.fn(mockFS), tt.wantErr)
		})
	}
}

func TestCertOrKeyExist(t *testing.T) {
	t.Parallel()

	t.Run("neither cert nor key exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		require.False(t, pki.CertOrKeyExist(fs, pkiDir, "absent"))
	})

	t.Run("only cert exists", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, _, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCert(fs, pkiDir, "certonly", caCert))
		require.True(t, pki.CertOrKeyExist(fs, pkiDir, "certonly"))
	})

	t.Run("only key exists", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteKey(fs, pkiDir, "keyonly", caKey))
		require.True(t, pki.CertOrKeyExist(fs, pkiDir, "keyonly"))
	})

	t.Run("both cert and key exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "both", caCert, caKey))
		require.True(t, pki.CertOrKeyExist(fs, pkiDir, "both"))
	})
}

func TestTryLoadCertFromDisk(t *testing.T) {
	t.Parallel()

	t.Run("loads written cert successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey))

		loaded, err := pki.TryLoadCertFromDisk(fs, pkiDir, "ca")
		require.NoError(t, err)
		require.NotNil(t, loaded)
		require.Equal(t, "test-ca", loaded.Subject.CommonName)
	})

	t.Run("returns error when cert does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, err := pki.TryLoadCertFromDisk(fs, pkiDir, "absent")
		require.Error(t, err)
	})

	t.Run("returns error for invalid PEM data", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		certPath, _ := pki.PathsForCertAndKey(pkiDir, "bad")
		require.NoError(t, fs.MkdirAll(pkiDir, 0o755))
		require.NoError(t, fs.WriteFile(certPath, []byte("not-valid-pem"), 0o644))

		_, err := pki.TryLoadCertFromDisk(fs, pkiDir, "bad")
		require.Error(t, err)
	})
}

func TestTryLoadKeyFromDisk(t *testing.T) {
	t.Parallel()

	t.Run("loads RSA key successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey))

		loaded, err := pki.TryLoadKeyFromDisk(fs, pkiDir, "ca")
		require.NoError(t, err)
		require.NotNil(t, loaded)
	})

	t.Run("loads ECDSA key successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmECDSAP256,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ec", caCert, caKey))

		loaded, err := pki.TryLoadKeyFromDisk(fs, pkiDir, "ec")
		require.NoError(t, err)
		require.NotNil(t, loaded)

		_, ok := loaded.(*ecdsa.PrivateKey)
		require.True(t, ok, "expected ECDSA private key")
	})

	t.Run("returns error when key does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, err := pki.TryLoadKeyFromDisk(fs, pkiDir, "absent")
		require.Error(t, err)
	})

	t.Run("returns error for invalid PEM data", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, keyPath := pki.PathsForCertAndKey(pkiDir, "bad")
		require.NoError(t, fs.MkdirAll(pkiDir, 0o755))
		require.NoError(t, fs.WriteFile(keyPath, []byte("not-valid-pem"), 0o600))

		_, err := pki.TryLoadKeyFromDisk(fs, pkiDir, "bad")
		require.Error(t, err)
	})

	t.Run("returns error for unsupported key type", func(t *testing.T) {
		t.Parallel()
		// Write an ed25519 key as PKCS8 PEM: ParsePrivateKeyPEM accepts it
		// but TryLoadKeyFromDisk only handles RSA and ECDSA, so the default branch fires.
		_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		keyBytes, err := x509.MarshalPKCS8PrivateKey(ed25519Key)
		require.NoError(t, err)
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

		fs := host.NewMemMapFS()
		_, keyPath := pki.PathsForCertAndKey(pkiDir, "ed25519")
		require.NoError(t, fs.MkdirAll(pkiDir, 0o755))
		require.NoError(t, fs.WriteFile(keyPath, keyPEM, 0o600))

		_, err = pki.TryLoadKeyFromDisk(fs, pkiDir, "ed25519")
		require.Error(t, err)
		require.Contains(t, err.Error(), "neither in RSA nor ECDSA format")
	})
}

func TestTryLoadCertAndKeyFromDisk(t *testing.T) {
	t.Parallel()

	t.Run("loads both cert and key successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey))

		loadedCert, loadedKey, err := pki.TryLoadCertAndKeyFromDisk(fs, pkiDir, "ca")
		require.NoError(t, err)
		require.NotNil(t, loadedCert)
		require.NotNil(t, loadedKey)
		require.Equal(t, "test-ca", loadedCert.Subject.CommonName)
	})

	t.Run("returns error when cert is missing", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteKey(fs, pkiDir, "keyonly", caKey))

		_, _, err = pki.TryLoadCertAndKeyFromDisk(fs, pkiDir, "keyonly")
		require.Error(t, err)
	})

	t.Run("returns error when key is missing", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, _, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCert(fs, pkiDir, "certonly", caCert))

		_, _, err = pki.TryLoadCertAndKeyFromDisk(fs, pkiDir, "certonly")
		require.Error(t, err)
	})
}

func TestPrivateKeyFromFile(t *testing.T) {
	t.Parallel()

	t.Run("reads RSA private key from file", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteKey(fs, pkiDir, "rsa", caKey))

		_, keyPath := pki.PathsForCertAndKey(pkiDir, "rsa")
		key, err := pki.PrivateKeyFromFile(fs, keyPath)
		require.NoError(t, err)
		require.NotNil(t, key)
	})

	t.Run("returns error when file does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, err := pki.PrivateKeyFromFile(fs, filepath.Join(pkiDir, "absent.key"))
		require.Error(t, err)
	})

	t.Run("returns error for invalid PEM", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		badPath := filepath.Join(pkiDir, "bad.key")
		require.NoError(t, fs.MkdirAll(pkiDir, 0o755))
		require.NoError(t, fs.WriteFile(badPath, []byte("not-a-key"), 0o600))

		_, err := pki.PrivateKeyFromFile(fs, badPath)
		require.Error(t, err)
	})
}

func TestCertsFromFile(t *testing.T) {
	t.Parallel()

	t.Run("reads certificate from file", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "test-ca"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca", caCert, caKey))

		certPath, _ := pki.PathsForCertAndKey(pkiDir, "ca")
		certs, err := pki.CertsFromFile(fs, certPath)
		require.NoError(t, err)
		require.Len(t, certs, 1)
		require.Equal(t, "test-ca", certs[0].Subject.CommonName)
	})

	t.Run("returns error when file does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		_, err := pki.CertsFromFile(fs, filepath.Join(pkiDir, "absent.crt"))
		require.Error(t, err)
	})

	t.Run("returns error for invalid PEM", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		badPath := filepath.Join(pkiDir, "bad.crt")
		require.NoError(t, fs.MkdirAll(pkiDir, 0o755))
		require.NoError(t, fs.WriteFile(badPath, []byte("not-a-cert"), 0o644))

		_, err := pki.CertsFromFile(fs, badPath)
		require.Error(t, err)
	})
}

func TestLoadX509KeyPair(t *testing.T) {
	t.Parallel()

	t.Run("loads RSA key pair successfully", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		newTestCACert(t, fs)

		certPath, keyPath := pki.PathsForCertAndKey(pkiDir, "ca")
		tlsCert, err := pki.LoadX509KeyPair(fs, certPath, keyPath)
		require.NoError(t, err)
		require.NotNil(t, tlsCert.Certificate)
	})

	t.Run("returns error when cert file does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		newTestCACert(t, fs)

		_, keyPath := pki.PathsForCertAndKey(pkiDir, "ca")
		_, err := pki.LoadX509KeyPair(fs, filepath.Join(pkiDir, "absent.crt"), keyPath)
		require.Error(t, err)
	})

	t.Run("returns error when key file does not exist", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()
		newTestCACert(t, fs)

		certPath, _ := pki.PathsForCertAndKey(pkiDir, "ca")
		_, err := pki.LoadX509KeyPair(fs, certPath, filepath.Join(pkiDir, "absent.key"))
		require.Error(t, err)
	})

	t.Run("returns error for mismatched cert and key", func(t *testing.T) {
		t.Parallel()
		fs := host.NewMemMapFS()

		cert1, key1, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "ca1"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteCertAndKey(fs, pkiDir, "ca1", cert1, key1))

		_, key2, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
			Config:              certutil.Config{CommonName: "ca2"},
			EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
		})
		require.NoError(t, err)
		require.NoError(t, pki.WriteKey(fs, pkiDir, "ca2", key2))

		certPath, _ := pki.PathsForCertAndKey(pkiDir, "ca1")
		_, keyPath := pki.PathsForCertAndKey(pkiDir, "ca2")
		_, err = pki.LoadX509KeyPair(fs, certPath, keyPath)
		require.Error(t, err)
	})
}
