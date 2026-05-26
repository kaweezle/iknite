package env

// cSpell: words pkgsecrets

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"path/filepath"
	"time"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	pkgsecrets "github.com/kaweezle/iknite/pkg/secrets"
)

const (
	dirMode       = 0o755
	fileMode      = 0o644
	defaultValues = `
apiVersion: config.iknite.app/v1alpha1
kind: PlatformValues
metadata:
  name: iknite-values
data:
    cloudProviders: {}
`
)

// InitRequest defines env init behavior.
type InitRequest struct {
	ConfigDir      string
	Force          bool
	NonInteractive bool
	PrintPaths     bool
}

// InitResult reports created paths and messages.
type InitResult struct {
	Paths    *ClientConfigPaths
	Messages []string
}

// Service initializes the iknitectl environment tree.
type Service struct {
	FS     host.FileEnvironment
	Logger *slog.Logger
	paths  *ClientConfigPaths
}

// Init creates required directories, secrets files, and default CA material.
func (s *Service) Init(req *InitRequest) (*InitResult, error) {
	if req == nil {
		req = &InitRequest{}
	}
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	if err := s.resolvePaths(req, s.paths); err != nil {
		return nil, err
	}

	if mkErr := ensureDirectoryTree(s.FS, s.paths); mkErr != nil {
		return nil, mkErr
	}

	secretsResult, err := initSecrets(s.FS, s.paths, req.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets: %w", err)
	}

	if err = ensureCertificateAuthority(s.FS, s.paths.CACert, s.paths.CAKey, req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize certificate authority: %w", err)
	}

	if err = ensureSharedValuesFile(s.FS, s.paths.SharedValues, req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize shared values file: %w", err)
	}

	messages := buildMessages(s.paths, secretsResult.Messages, req.PrintPaths)

	return &InitResult{Paths: s.paths, Messages: messages}, nil
}

func (s *Service) ensureDefaults() error {
	if s.FS == nil {
		return fmt.Errorf("filesystem dependency is required")
	}

	if s.Logger == nil {
		return fmt.Errorf("logger dependency is required")
	}

	if s.paths == nil {
		s.paths = &ClientConfigPaths{}
	}

	return nil
}

func (s *Service) resolvePaths(req *InitRequest, paths *ClientConfigPaths) error {
	configDir := req.ConfigDir

	if configDir == "" {
		var err error
		configDir, err = defaultConfigDir(s.FS)
		if err != nil {
			return fmt.Errorf("failed to get default config directory: %w", err)
		}
	}
	configDir = filepath.Clean(configDir)
	paths.Root = configDir

	paths.Auth = s.FS.JoinPath(configDir, defaultAuthDirname)
	paths.Shared = s.FS.JoinPath(configDir, defaultSharedDirname)
	paths.Images = s.FS.JoinPath(configDir, defaultImagesDirname)
	paths.Clusters = s.FS.JoinPath(configDir, defaultClustersDirname)
	paths.CACert = filepath.Join(paths.Auth, defaultCACertFilename)
	paths.CAKey = filepath.Join(paths.Auth, defaultCAKeyFilename)

	paths.SharedSecrets = filepath.Join(paths.Shared, defaultSecretsFilename)
	paths.SharedSecretsKey = filepath.Join(paths.Shared, defaultKeyFilename)
	paths.SharedValues = filepath.Join(paths.Shared, defaultValuesFilename)

	return nil
}

func ensureDirectoryTree(fs host.FileSystem, paths *ClientConfigPaths) error {
	for _, path := range []string{paths.Root, paths.Auth, paths.Shared, paths.Images, paths.Clusters} {
		if err := fs.MkdirAll(path, dirMode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	return nil
}

func initSecrets(
	fs host.FileEnvironment,
	paths *ClientConfigPaths,
	force bool,
) (*pkgsecrets.InitResult, error) {
	secretsOpts := &pkgsecrets.Options{
		Fs:          fs,
		Force:       force,
		KeyFile:     paths.SharedSecretsKey,
		SecretsFile: paths.SharedSecrets,
	}

	result, err := pkgsecrets.InitSecrets(secretsOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets files: %w", err)
	}

	return result, nil
}

func buildMessages(paths *ClientConfigPaths, secretMessages []string, printPaths bool) []string {
	messages := []string{fmt.Sprintf("initialized iknitectl environment at %s", paths.Root)}
	messages = append(messages, secretMessages...)
	if printPaths {
		messages = append(
			messages,
			fmt.Sprintf("auth=%s", paths.Auth),
			fmt.Sprintf("shared=%s", paths.Shared),
			fmt.Sprintf("images=%s", paths.Images),
			fmt.Sprintf("clusters=%s", paths.Clusters),
		)
	}

	return messages
}

func defaultConfigDir(fse host.FileEnvironment) (string, error) {
	configDir, err := fse.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return fse.JoinPath(configDir, constants.IkniteConfName), nil
}

func ensureSharedValuesFile(fs host.FileSystem, path string, force bool) error {
	exists, err := fs.Exists(path)
	if err != nil {
		return fmt.Errorf("failed to check shared values file: %w", err)
	}
	if exists && !force {
		return nil
	}

	content := []byte(defaultValues)
	if err = fs.WriteFile(path, content, fileMode); err != nil {
		return fmt.Errorf("failed to write shared values file: %w", err)
	}

	return nil
}

func ensureCertificateAuthority(fs host.FileSystem, certPath, keyPath string, force bool) error {
	certExists, err := fs.Exists(certPath)
	if err != nil {
		return fmt.Errorf("failed to check CA certificate path: %w", err)
	}
	keyExists, err := fs.Exists(keyPath)
	if err != nil {
		return fmt.Errorf("failed to check CA key path: %w", err)
	}

	if certExists && keyExists && !force {
		return nil
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := notBefore.AddDate(10, 0, 0)
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate CA serial number: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "iknite-local-ca",
			Organization: []string{"iknitectl"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err = fs.WriteFile(certPath, certBytes, fileMode); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal CA private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	if err = fs.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("failed to write CA private key: %w", err)
	}

	return nil
}
