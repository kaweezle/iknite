// cSpell: words getsops sopsage
/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

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
package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	sopsage "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/config"
	"github.com/getsops/sops/v3/version"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"
)

const (
	DefaultSecretsFile = "secrets.sops.yaml" //nolint:gosec // Just a filename, not a credential
)

// Options contains configuration for secrets operations.
type Options struct {
	Fs          afero.Fs
	SecretsFile string
	HomeDir     string
	KeyFile     string
	Force       bool
}

// InitResult contains messages produced during secrets init.
type InitResult struct {
	Messages []string
}

// GetSecret retrieves a secret from the SOPS file for a dot-notated path.
func GetSecret(opts *Options, path string) (string, error) {
	fileSecrets, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return "", err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return "", err
	}

	tree := fileSecrets.Tree

	if len(tree.Branches) == 0 {
		return "", fmt.Errorf("secrets file has no data")
	}

	value, err := tree.Branches[0].Truncate(fullPath)
	if err != nil {
		return "", fmt.Errorf("secret path %q not found: %w", path, err)
	}

	switch typed := value.(type) {
	case string:
		return typed, nil
	default:
		yamlData, marshalErr := yaml.Marshal(typed)
		if marshalErr != nil {
			return "", fmt.Errorf("failed to marshal value at %q: %w", path, marshalErr)
		}
		return strings.TrimRight(string(yamlData), "\n"), nil
	}
}

// SetSecret sets a secret in the SOPS file for a dot-notated path.
func SetSecret(opts *Options, path, value string) error {
	fileSecrets, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return err
	}

	tree := fileSecrets.Tree

	if len(tree.Branches) == 0 {
		tree.Branches = sops.TreeBranches{{}}
	}

	tree.Branches[0], _ = tree.Branches[0].Set(fullPath, value)

	if err = common.EncryptTree(
		common.EncryptTreeOpts{Tree: tree, Cipher: aes.NewCipher(), DataKey: fileSecrets.DataKey},
	); err != nil {
		return fmt.Errorf("failed to encrypt updated secrets: %w", err)
	}

	encryptedData, err := fileSecrets.Store.EmitEncryptedFile(*tree)
	if err != nil {
		return fmt.Errorf("failed to encode encrypted secrets: %w", err)
	}

	if writeErr := afero.WriteFile(opts.Fs, opts.SecretsFile, encryptedData, fileSecrets.Mode); writeErr != nil {
		return fmt.Errorf("failed to write secrets file: %w", writeErr)
	}

	return nil
}

// RemoveSecret removes a secret from the SOPS file for a dot-notated path.
func RemoveSecret(opts *Options, path string) error {
	fileSecrets, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return err
	}

	tree := fileSecrets.Tree

	if len(tree.Branches) == 0 {
		return fmt.Errorf("secret path %q not found", path)
	}

	updatedBranch, err := tree.Branches[0].Unset(fullPath)
	if err != nil {
		return fmt.Errorf("secret path %q not found: %w", path, err)
	}
	tree.Branches[0] = updatedBranch

	if err = common.EncryptTree(
		common.EncryptTreeOpts{Tree: tree, Cipher: aes.NewCipher(), DataKey: fileSecrets.DataKey},
	); err != nil {
		return fmt.Errorf("failed to encrypt updated secrets: %w", err)
	}

	encryptedData, err := fileSecrets.Store.EmitEncryptedFile(*tree)
	if err != nil {
		return fmt.Errorf("failed to encode encrypted secrets: %w", err)
	}

	if writeErr := afero.WriteFile(opts.Fs, opts.SecretsFile, encryptedData, fileSecrets.Mode); writeErr != nil {
		return fmt.Errorf("failed to write secrets file: %w", writeErr)
	}

	return nil
}

func checkSecretsFilesExists(opts *Options, paths *secretsInitPaths, result *InitResult) error {
	if exists, existsErr := afero.Exists(opts.Fs, paths.sopsConfigFile); existsErr != nil {
		return fmt.Errorf("failed to check .sops.yaml: %w", existsErr)
	} else if exists {
		result.Messages = append(
			result.Messages,
			fmt.Sprintf("%s already exists, not overwriting", paths.sopsConfigFile),
		)
	}

	if exists, existsErr := afero.Exists(opts.Fs, paths.secretsFile); existsErr != nil {
		return fmt.Errorf("failed to check secrets file: %w", existsErr)
	} else if exists {
		result.Messages = append(
			result.Messages,
			fmt.Sprintf("%s already exists, not overwriting", paths.secretsFile),
		)
	}
	return nil
}

// InitSecrets initializes SOPS config, encrypted secrets, and an SSH key pair.
func InitSecrets(opts *Options) (*InitResult, error) {
	result := &InitResult{}

	paths, err := resolveSecretsInitPaths(opts)
	if err != nil {
		return nil, err
	}

	if !opts.Force {
		if err = checkSecretsFilesExists(opts, paths, result); err != nil {
			return nil, fmt.Errorf("failed to check existing secrets files: %w", err)
		}
		if len(result.Messages) > 0 {
			return result, nil
		}
	}

	keyInfo, err := ensureSSHKeyPair(opts.Fs, paths.keyFile, paths.publicKeyFile)
	if err != nil {
		return nil, err
	}
	if keyInfo.Generated {
		result.Messages = append(
			result.Messages,
			fmt.Sprintf("Generated new SSH key pair at %s and %s", paths.displayKeyFile, paths.displayPublicKeyFile),
		)
	} else if keyInfo.AuthorizedKeyDerived {
		result.Messages = append(
			result.Messages,
			fmt.Sprintf(
				"Derived public key from existing private key at %s and wrote to %s",
				paths.displayKeyFile,
				paths.displayPublicKeyFile,
			),
		)
	} else {
		result.Messages = append(
			result.Messages,
			fmt.Sprintf("Using existing SSH key pair at %s and %s", paths.displayKeyFile, paths.displayPublicKeyFile),
		)
	}

	templateData := &TemplateData{
		Values: map[string]any{
			"publicKeyPath":  paths.displayPublicKeyFile,
			"publicKey":      keyInfo.AuthorizedKey,
			"privateKeyPath": paths.displayKeyFile,
			"privateKey":     keyInfo.PrivateKeyPEM,
		},
	}

	var sopsConfig string
	sopsConfig, err = renderTemplate(SopsConfigTemplateName, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render SOPS config template: %w", err)
	}
	if writeErr := afero.WriteFile(opts.Fs, paths.sopsConfigFile, []byte(sopsConfig), 0o644); writeErr != nil {
		return nil, fmt.Errorf("failed to write %s: %w", paths.sopsConfigFile, writeErr)
	}
	result.Messages = append(result.Messages, fmt.Sprintf("Wrote %s", paths.sopsConfigFile))

	var plaintextSecrets string
	plaintextSecrets, err = renderTemplate(SecretsTemplateName, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render secrets template: %w", err)
	}

	encryptedSecrets, err := encryptSecretsPlaintext(paths.secretsFile, []byte(plaintextSecrets), keyInfo.AuthorizedKey)
	if err != nil {
		return nil, err
	}
	if writeErr := afero.WriteFile(opts.Fs, paths.secretsFile, encryptedSecrets, 0o644); writeErr != nil {
		return nil, fmt.Errorf("failed to write %s: %w", paths.secretsFile, writeErr)
	}
	result.Messages = append(result.Messages, fmt.Sprintf("Wrote %s", paths.secretsFile))

	if keyInfo.Generated && opts.KeyFile != "" {
		result.Messages = append(result.Messages,
			fmt.Sprintf("Set SOPS_AGE_SSH_PRIVATE_KEY_FILE=%s to decrypt the generated secrets file", paths.keyFile),
		)
	}

	return result, nil
}

type FileSecrets struct {
	Store   common.Store
	Tree    *sops.Tree
	DataKey []byte
	Mode    os.FileMode
}

func loadAndDecryptSecrets(opts *Options) (*FileSecrets, error) {
	exists, err := afero.Exists(opts.Fs, opts.SecretsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check secrets file: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("secrets file not found: %s", opts.SecretsFile)
	}

	fileInfo, err := opts.Fs.Stat(opts.SecretsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat secrets file: %w", err)
	}

	encryptedData, err := afero.ReadFile(opts.Fs, opts.SecretsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	format := formats.FormatForPathOrString(opts.SecretsFile, "")
	store := common.StoreForFormat(format, config.NewStoresConfig())
	tree, err := store.LoadEncryptedFile(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse encrypted secrets: %w", err)
	}

	dataKey, err := tree.Metadata.GetDataKey()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve data key: %w", err)
	}

	if _, err := tree.Decrypt(dataKey, aes.NewCipher()); err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets file: %w", err)
	}

	return &FileSecrets{
		Store:   store,
		Tree:    &tree,
		DataKey: dataKey,
		Mode:    fileInfo.Mode().Perm(),
	}, nil
}

func buildSecretsPath(path string) ([]any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("secret path cannot be empty")
	}

	fullPath := make([]any, 0, len(parts)+1)
	fullPath = append(fullPath, "data")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("invalid secret path %q", path)
		}
		fullPath = append(fullPath, trimmed)
	}

	return fullPath, nil
}

type secretsInitPaths struct {
	secretsFile          string
	sopsConfigFile       string
	keyFile              string
	publicKeyFile        string
	displayKeyFile       string
	displayPublicKeyFile string
}

func resolveSecretsInitPaths(opts *Options) (*secretsInitPaths, error) {
	secretsFile := opts.SecretsFile
	if secretsFile == "" {
		secretsFile = DefaultSecretsFile
	}

	secretsDir := filepath.Dir(secretsFile)
	if secretsDir == "" {
		secretsDir = "."
	}

	keyFile := opts.KeyFile
	if keyFile == "" {
		if opts.HomeDir == "" {
			return nil, fmt.Errorf("home directory is required to determine the default key file")
		}
		keyFile = filepath.Join(opts.HomeDir, ".ssh", "id_ed25519")
	}
	keyFile = expandHomePath(opts.HomeDir, keyFile)

	publicKeyFile := keyFile + ".pub"

	return &secretsInitPaths{
		secretsFile:          secretsFile,
		sopsConfigFile:       filepath.Join(secretsDir, ".sops.yaml"),
		keyFile:              keyFile,
		publicKeyFile:        publicKeyFile,
		displayKeyFile:       displayPath(opts.HomeDir, keyFile),
		displayPublicKeyFile: displayPath(opts.HomeDir, publicKeyFile),
	}, nil
}

func encryptSecretsPlaintext(secretsFile string, plaintext []byte, recipient string) ([]byte, error) {
	store := common.StoreForFormat(formats.FormatForPathOrString(secretsFile, ""), config.NewStoresConfig())
	branches, err := store.LoadPlainFile(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plaintext secrets template: %w", err)
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("plaintext secrets template produced no YAML documents")
	}

	masterKeys, err := sopsage.MasterKeysFromRecipients(recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recipient %q: %w", recipient, err)
	}
	keyGroup := make(sops.KeyGroup, 0, len(masterKeys))
	for _, key := range masterKeys {
		keyGroup = append(keyGroup, key)
	}

	tree := sops.Tree{
		Branches: branches,
		Metadata: sops.Metadata{
			KeyGroups:      []sops.KeyGroup{keyGroup},
			EncryptedRegex: "^data$",
			Version:        version.Version,
		},
		FilePath: secretsFile,
	}

	dataKey, errs := tree.GenerateDataKey()
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to generate data key: %v", errs)
	}

	if err = common.EncryptTree(
		common.EncryptTreeOpts{Tree: &tree, Cipher: aes.NewCipher(), DataKey: dataKey},
	); err != nil {
		return nil, fmt.Errorf("failed to encrypt initial secrets file: %w", err)
	}

	encryptedData, err := store.EmitEncryptedFile(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to encode encrypted secrets file: %w", err)
	}

	return encryptedData, nil
}

func expandHomePath(homeDir, path string) string {
	if homeDir == "" {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func displayPath(homeDir, path string) string {
	if homeDir == "" {
		return path
	}
	relativeToHome, err := filepath.Rel(homeDir, path)
	if err == nil && relativeToHome != "." && relativeToHome != ".." &&
		!strings.HasPrefix(relativeToHome, ".."+string(filepath.Separator)) {
		return filepath.Join("~", relativeToHome)
	}
	if path == homeDir {
		return "~"
	}
	return path
}
