// cSpell: words certutil kubeadmapi pkiutil
package config

import (
	"fmt"
	"log/slog"
	"os"

	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
)

const (
	dirMode = 0o755
	dbMode  = 0o600

	DefaultAuthDirname     = "auth"
	DefaultSharedDirname   = "shared"
	DefaultImagesDirname   = "images"
	DefaultClustersDirname = "clusters"
	DefaultDatabaseFile    = "iknite.db"
	DefaultValuesFilename  = "values.yaml"
	DefaultKeyFilename     = "id_ed25519"
	DefaultSecretsFilename = "secrets.sops.yaml" //nolint:gosec // This is just a filename.
)

type CertificateAuthorityConfig struct {
	CommonName   string
	CertPath     string
	KeyPath      string
	Organization []string
}

type Config struct {
	Root     string
	Auth     string
	Shared   string
	Images   string
	Clusters string
	Database string

	SharedSecrets    string
	SharedSecretsKey string
	SharedValues     string

	CA CertificateAuthorityConfig
}

type ConfigProvider interface {
	Config() *Config
}

func (c *Config) EnsureDirectoryTree(fs host.FileSystem) error {
	for _, path := range []string{c.Root, c.Auth, c.Shared, c.Images, c.Clusters} {
		if err := fs.MkdirAll(path, dirMode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	return nil
}

func (c *Config) EnsureCertificateAuthority(fs host.FileSystem, logger *slog.Logger, force bool) error {
	certExists, err := fs.Exists(c.CA.CertPath)
	if err != nil {
		return fmt.Errorf("failed to check CA certificate path: %w", err)
	}
	keyExists, err := fs.Exists(c.CA.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to check CA key path: %w", err)
	}

	if certExists && keyExists && !force {
		logger.Info("CA certificate and key already exist, skipping generation",
			"certPath", c.CA.CertPath, "keyPath", c.CA.KeyPath)
		return nil
	}

	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   c.CA.CommonName,
			Organization: c.CA.Organization,
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	if err != nil {
		return fmt.Errorf("failed to generate CA certificate and key: %w", err)
	}

	err = pki.WriteCert(fs, c.CA.CertPath, caCert)
	if err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	err = pki.WriteKey(fs, c.CA.KeyPath, caKey)
	if err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	logger.Info("Generated new CA certificate and key", "certPath", c.CA.CertPath, "keyPath", c.CA.KeyPath)
	return nil
}

func (c *Config) EnsureDatabase(fs host.FileSystem) error {
	exists, err := fs.Exists(c.Database)
	if err != nil {
		return fmt.Errorf("failed to check database file: %w", err)
	}
	if exists {
		return nil
	}

	dbFile, err := fs.OpenFile(c.Database, os.O_CREATE|os.O_RDWR, dbMode)
	if err != nil {
		return fmt.Errorf("failed to initialize database file: %w", err)
	}
	if err = dbFile.Close(); err != nil {
		return fmt.Errorf("failed to close database file: %w", err)
	}

	return nil
}
