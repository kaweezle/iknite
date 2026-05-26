// cSpell: words pkgsecrets pkiutil certutil kubeadmapi
package env

import (
	"fmt"
	"log/slog"
	"path/filepath"

	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	pkiutil "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/pki"
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
	caCommonName = "iknite-local-ca"
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

	if err = s.ensureCertificateAuthority(req.Force); err != nil {
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

func (s *Service) ensureCertificateAuthority(force bool) error {
	certExists, err := s.FS.Exists(s.paths.CACert)
	if err != nil {
		return fmt.Errorf("failed to check CA certificate path: %w", err)
	}
	keyExists, err := s.FS.Exists(s.paths.CAKey)
	if err != nil {
		return fmt.Errorf("failed to check CA key path: %w", err)
	}

	if certExists && keyExists && !force {
		s.Logger.Info("CA certificate and key already exist, skipping generation",
			"certPath", s.paths.CACert, "keyPath", s.paths.CAKey)
		return nil
	}

	caCert, caKey, err := pkiutil.NewCertificateAuthority(&pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   caCommonName,
			Organization: []string{"iknitectl"},
		},
		EncryptionAlgorithm: kubeadmapi.EncryptionAlgorithmRSA2048,
	})
	if err != nil {
		return fmt.Errorf("failed to generate CA certificate and key: %w", err)
	}

	err = pki.WriteCert(s.FS, s.paths.CACert, caCert)
	if err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	err = pki.WriteKey(s.FS, s.paths.CAKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	s.Logger.Info("Generated new CA certificate and key", "certPath", s.paths.CACert, "keyPath", s.paths.CAKey)
	return nil
}
