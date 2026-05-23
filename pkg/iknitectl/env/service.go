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
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kaweezle/iknite/pkg/host"
	pkgsecrets "github.com/kaweezle/iknite/pkg/secrets"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
)

// EnvironmentProvider provides access to process environment variables.
type EnvironmentProvider interface {
	Getenv(key string) string
}

// PlatformDetector provides OS detection.
type PlatformDetector interface {
	GOOS() string
}

// InitRequest defines env init behavior.
type InitRequest struct {
	ConfigDir      string
	Force          bool
	NonInteractive bool
	PrintPaths     bool
}

// InitResult reports created paths and messages.
type InitResult struct {
	ConfigDir string
	Paths     map[string]string
	Messages  []string
}

// Service initializes the iknitectl environment tree.
type Service struct {
	FS       host.FileSystem
	Env      EnvironmentProvider
	Platform PlatformDetector
	HomeDir  func() (string, error)
}

// Init creates required directories, secrets files, and default CA material.
func (s *Service) Init(req *InitRequest) (*InitResult, error) {
	if req == nil {
		req = &InitRequest{}
	}
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	homeDir, configDir, paths, err := s.resolvePaths(req)
	if err != nil {
		return nil, err
	}

	if mkErr := ensureDirectoryTree(s.FS, paths); mkErr != nil {
		return nil, mkErr
	}

	secretsResult, err := initSecrets(s.FS, homeDir, paths, req.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets: %w", err)
	}

	paths["caCert"] = filepath.Join(paths["auth"], "ca.crt")
	paths["caKey"] = filepath.Join(paths["auth"], "ca.key")
	if err = ensureCertificateAuthority(s.FS, paths["caCert"], paths["caKey"], req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize certificate authority: %w", err)
	}

	paths["sharedValues"] = filepath.Join(paths["shared"], "values.yaml")
	if err = ensureSharedValuesFile(s.FS, paths["sharedValues"], req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize shared values file: %w", err)
	}

	messages := buildMessages(paths, secretsResult.Messages, req.PrintPaths)

	return &InitResult{ConfigDir: configDir, Paths: paths, Messages: messages}, nil
}

func (s *Service) ensureDefaults() error {
	if s.FS == nil {
		return fmt.Errorf("filesystem dependency is required")
	}
	if s.Env == nil {
		s.Env = osEnvironmentProvider{}
	}
	if s.Platform == nil {
		s.Platform = runtimePlatformDetector{}
	}
	if s.HomeDir == nil {
		s.HomeDir = os.UserHomeDir
	}

	return nil
}

func (s *Service) resolvePaths(req *InitRequest) (string, string, map[string]string, error) {
	homeDir, err := s.HomeDir()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to resolve home directory: %w", err)
	}

	configDir := req.ConfigDir
	if configDir == "" {
		configDir, err = defaultConfigDir(s.Env, s.Platform, homeDir)
		if err != nil {
			return "", "", nil, err
		}
	}
	configDir = filepath.Clean(configDir)

	paths := map[string]string{
		"root":     configDir,
		"auth":     filepath.Join(configDir, "auth"),
		"shared":   filepath.Join(configDir, "shared"),
		"images":   filepath.Join(configDir, "images"),
		"clusters": filepath.Join(configDir, "clusters"),
	}

	return homeDir, configDir, paths, nil
}

func ensureDirectoryTree(fs host.FileSystem, paths map[string]string) error {
	for _, path := range []string{paths["root"], paths["auth"], paths["shared"], paths["images"], paths["clusters"]} {
		if err := fs.MkdirAll(path, dirMode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	return nil
}

func initSecrets(
	fs host.FileSystem,
	homeDir string,
	paths map[string]string,
	force bool,
) (*pkgsecrets.InitResult, error) {
	secretsOpts := &pkgsecrets.Options{
		Fs:          fs,
		HomeDir:     homeDir,
		Force:       force,
		KeyFile:     filepath.Join(paths["auth"], "id_ed25519"),
		SecretsFile: filepath.Join(paths["shared"], pkgsecrets.DefaultSecretsFile),
	}

	result, err := pkgsecrets.InitSecrets(secretsOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets files: %w", err)
	}

	return result, nil
}

func buildMessages(paths map[string]string, secretMessages []string, printPaths bool) []string {
	messages := []string{fmt.Sprintf("initialized iknitectl environment at %s", paths["root"])}
	messages = append(messages, secretMessages...)
	if printPaths {
		for key, path := range paths {
			messages = append(messages, fmt.Sprintf("%s=%s", key, path))
		}
	}

	return messages
}

func defaultConfigDir(env EnvironmentProvider, platform PlatformDetector, homeDir string) (string, error) {
	if homeDir == "" {
		return "", fmt.Errorf("home directory is required")
	}

	switch platform.GOOS() {
	case "windows":
		if appData := env.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "iknite"), nil
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "iknite"), nil
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "iknite"), nil
	default:
		if xdgConfigHome := env.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
			return filepath.Join(xdgConfigHome, "iknite"), nil
		}
		return filepath.Join(homeDir, ".config", "iknite"), nil
	}
}

func ensureSharedValuesFile(fs host.FileSystem, path string, force bool) error {
	exists, err := fs.Exists(path)
	if err != nil {
		return fmt.Errorf("failed to check shared values file: %w", err)
	}
	if exists && !force {
		return nil
	}

	content := []byte("shared:\n  backend: {}\n")
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

type osEnvironmentProvider struct{}

func (osEnvironmentProvider) Getenv(key string) string {
	return os.Getenv(key)
}

type runtimePlatformDetector struct{}

func (runtimePlatformDetector) GOOS() string {
	return runtime.GOOS
}
