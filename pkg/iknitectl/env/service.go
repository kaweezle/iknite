// cSpell: words pkgsecrets pkiutil certutil kubeadmapi
package env

import (
	"fmt"
	"log/slog"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	pkgsecrets "github.com/kaweezle/iknite/pkg/secrets"
)

const (
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
	Force      bool
	PrintPaths bool
}

// InitResult reports created paths and messages.
type InitResult struct {
	Paths    *config.Config
	Messages []string
}

// Service initializes the iknitectl environment tree.
type Service struct {
	FS     host.FileEnvironment
	Logger *slog.Logger
	Config *config.Config
}

// Init creates required directories, secrets files, and default CA material.
func (s *Service) Init(req *InitRequest) (*InitResult, error) {
	if req == nil {
		req = &InitRequest{}
	}
	if err := s.ensureDefaults(); err != nil {
		return nil, err
	}

	if mkErr := s.Config.EnsureDirectoryTree(s.FS); mkErr != nil {
		return nil, fmt.Errorf("failed to create config directory tree: %w", mkErr)
	}

	if dbErr := s.Config.EnsureDatabase(s.FS); dbErr != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", dbErr)
	}

	secretsResult, err := initSecrets(s.FS, s.Config, req.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets: %w", err)
	}

	if err = s.Config.EnsureCertificateAuthority(s.FS, s.Logger, req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize certificate authority: %w", err)
	}

	if err = ensureSharedValuesFile(s.FS, s.Config.SharedValues, req.Force); err != nil {
		return nil, fmt.Errorf("failed to initialize shared values file: %w", err)
	}

	messages := buildMessages(s.Config, secretsResult.Messages, req.PrintPaths)

	return &InitResult{Paths: s.Config, Messages: messages}, nil
}

func (s *Service) ensureDefaults() error {
	if s.FS == nil {
		return fmt.Errorf("filesystem dependency is required")
	}

	if s.Logger == nil {
		return fmt.Errorf("logger dependency is required")
	}

	if s.Config == nil {
		c, err := config.NewDefaultConfig(s.FS)
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		s.Config = c
	}

	return nil
}

func initSecrets(
	fs host.FileEnvironment,
	paths *config.Config,
	force bool,
) (*pkgsecrets.InitResult, error) {
	secretsOpts := &pkgsecrets.Options{
		Fs:           fs,
		Force:        force,
		OverwriteKey: force,
		KeyFile:      paths.SharedSecretsKey,
		SecretsFile:  paths.SharedSecrets,
	}

	result, err := pkgsecrets.InitSecrets(secretsOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets files: %w", err)
	}

	return result, nil
}

func buildMessages(paths *config.Config, secretMessages []string, printPaths bool) []string {
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
