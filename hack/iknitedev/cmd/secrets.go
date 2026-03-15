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
package cmd

import (
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
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
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/yaml"
)

// SecretsOptions contains configuration for the secrets command.
type SecretsOptions struct {
	Fs          afero.Fs
	SecretsFile string
	In          io.Reader
	HomeDir     string
	KeyFile     string
	Force       bool
}

// SecretsInitResult contains messages produced during secrets init.
type SecretsInitResult struct {
	Messages []string
}

// CreateSecretsCmd creates the secrets command.
func CreateSecretsCmd(fs afero.Fs, opts *SecretsOptions) *cobra.Command {
	if opts == nil {
		opts = &SecretsOptions{}
	}
	if opts.Fs == nil {
		opts.Fs = fs
	}
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.HomeDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			opts.HomeDir = homeDir
		}
	}
	if opts.SecretsFile == "" {
		opts.SecretsFile = "secrets.sops.yaml"
	}

	defaultSecretsFile := opts.SecretsFile

	secretsCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Read and modify values in a SOPS secrets file",
		Long: `Read and modify values in a SOPS encrypted secrets file.

Paths are specified in dot notation under the data key.
For example, github.api_token targets data.github.api_token.`,
	}

	secretsCmd.PersistentFlags().StringVarP(
		&opts.SecretsFile,
		"secrets-file",
		"s",
		defaultSecretsFile,
		"Path to the SOPS secrets file",
	)

	secretsCmd.AddCommand(createSecretsGetCmd(opts))
	secretsCmd.AddCommand(createSecretsSetCmd(opts))
	secretsCmd.AddCommand(createSecretsRemoveCmd(opts))
	secretsCmd.AddCommand(createSecretsInitCmd(opts))

	return secretsCmd
}

func createSecretsGetCmd(opts *SecretsOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <path>",
		Short: "Get a secret value from the secrets file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := GetSecret(opts, args[0])
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
			return err
		},
	}
}

func createSecretsSetCmd(opts *SecretsOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> [value]",
		Short: "Set a secret value in the secrets file",
		Long: `Set a secret value in the secrets file.

When value is omitted, it is read from stdin.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			value := ""
			if len(args) == 2 {
				value = args[1]
			} else {
				data, err := io.ReadAll(opts.In)
				if err != nil {
					return fmt.Errorf("failed to read value from stdin: %w", err)
				}
				value = strings.TrimRight(string(data), "\r\n")
			}

			return SetSecret(opts, args[0], value)
		},
	}
}

func createSecretsRemoveCmd(opts *SecretsOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a secret key from the secrets file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return RemoveSecret(opts, args[0])
		},
	}
}

func createSecretsInitCmd(opts *SecretsOptions) *cobra.Command {
	defaultKeyFile := opts.KeyFile
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .sops.yaml, secrets.sops.yaml, and an SSH key pair",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := InitSecrets(opts)
			if err != nil {
				return err
			}

			for _, message := range result.Messages {
				if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), message); writeErr != nil {
					return writeErr
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Overwrite existing generated files")
	cmd.Flags().StringVarP(&opts.KeyFile, "key-file", "k", defaultKeyFile, "SSH private key file to use or generate")

	return cmd
}

// GetSecret retrieves a secret from the SOPS file for a dot-notated path.
func GetSecret(opts *SecretsOptions, path string) (string, error) {
	_, tree, _, _, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return "", err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return "", err
	}

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
func SetSecret(opts *SecretsOptions, path string, value string) error {
	store, tree, dataKey, mode, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return err
	}

	if len(tree.Branches) == 0 {
		tree.Branches = sops.TreeBranches{{}}
	}

	tree.Branches[0], _ = tree.Branches[0].Set(fullPath, value)

	if err := common.EncryptTree(common.EncryptTreeOpts{Tree: tree, Cipher: aes.NewCipher(), DataKey: dataKey}); err != nil {
		return fmt.Errorf("failed to encrypt updated secrets: %w", err)
	}

	encryptedData, err := store.EmitEncryptedFile(*tree)
	if err != nil {
		return fmt.Errorf("failed to encode encrypted secrets: %w", err)
	}

	if writeErr := afero.WriteFile(opts.Fs, opts.SecretsFile, encryptedData, mode); writeErr != nil {
		return fmt.Errorf("failed to write secrets file: %w", writeErr)
	}

	return nil
}

// RemoveSecret removes a secret from the SOPS file for a dot-notated path.
func RemoveSecret(opts *SecretsOptions, path string) error {
	store, tree, dataKey, mode, err := loadAndDecryptSecrets(opts)
	if err != nil {
		return err
	}

	fullPath, err := buildSecretsPath(path)
	if err != nil {
		return err
	}

	if len(tree.Branches) == 0 {
		return fmt.Errorf("secret path %q not found", path)
	}

	updatedBranch, err := tree.Branches[0].Unset(fullPath)
	if err != nil {
		return fmt.Errorf("secret path %q not found: %w", path, err)
	}
	tree.Branches[0] = updatedBranch

	if err := common.EncryptTree(common.EncryptTreeOpts{Tree: tree, Cipher: aes.NewCipher(), DataKey: dataKey}); err != nil {
		return fmt.Errorf("failed to encrypt updated secrets: %w", err)
	}

	encryptedData, err := store.EmitEncryptedFile(*tree)
	if err != nil {
		return fmt.Errorf("failed to encode encrypted secrets: %w", err)
	}

	if writeErr := afero.WriteFile(opts.Fs, opts.SecretsFile, encryptedData, mode); writeErr != nil {
		return fmt.Errorf("failed to write secrets file: %w", writeErr)
	}

	return nil
}

// InitSecrets initializes SOPS config, encrypted secrets, and an SSH key pair.
func InitSecrets(opts *SecretsOptions) (*SecretsInitResult, error) {
	result := &SecretsInitResult{}

	paths, err := resolveSecretsInitPaths(opts)
	if err != nil {
		return nil, err
	}

	if !opts.Force {
		if exists, existsErr := afero.Exists(opts.Fs, paths.sopsConfigFile); existsErr != nil {
			return nil, fmt.Errorf("failed to check .sops.yaml: %w", existsErr)
		} else if exists {
			result.Messages = append(result.Messages, fmt.Sprintf("%s already exists, not overwriting", paths.sopsConfigFile))
		}

		if exists, existsErr := afero.Exists(opts.Fs, paths.secretsFile); existsErr != nil {
			return nil, fmt.Errorf("failed to check secrets file: %w", existsErr)
		} else if exists {
			result.Messages = append(result.Messages, fmt.Sprintf("%s already exists, not overwriting", paths.secretsFile))
		}

		if len(result.Messages) > 0 {
			return result, nil
		}
	}

	keyInfo, err := ensureSSHKeyPair(opts.Fs, paths.keyFile, paths.publicKeyFile)
	if err != nil {
		return nil, err
	}

	sopsConfig := renderSOPSConfig(paths.displayPublicKeyFile, keyInfo.AuthorizedKey)
	if writeErr := afero.WriteFile(opts.Fs, paths.sopsConfigFile, []byte(sopsConfig), 0o644); writeErr != nil {
		return nil, fmt.Errorf("failed to write %s: %w", paths.sopsConfigFile, writeErr)
	}
	result.Messages = append(result.Messages, fmt.Sprintf("Wrote %s", paths.sopsConfigFile))

	plaintextSecrets := renderPlainSecretsFile(paths.displayPublicKeyFile, paths.displayKeyFile, keyInfo.AuthorizedKey, keyInfo.PrivateKeyPEM)
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

func loadAndDecryptSecrets(opts *SecretsOptions) (common.Store, *sops.Tree, []byte, os.FileMode, error) {
	exists, err := afero.Exists(opts.Fs, opts.SecretsFile)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to check secrets file: %w", err)
	}
	if !exists {
		return nil, nil, nil, 0, fmt.Errorf("secrets file not found: %s", opts.SecretsFile)
	}

	fileInfo, err := opts.Fs.Stat(opts.SecretsFile)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to stat secrets file: %w", err)
	}

	encryptedData, err := afero.ReadFile(opts.Fs, opts.SecretsFile)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to read secrets file: %w", err)
	}

	format := formats.FormatForPathOrString(opts.SecretsFile, "")
	store := common.StoreForFormat(format, config.NewStoresConfig())
	tree, err := store.LoadEncryptedFile(encryptedData)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to parse encrypted secrets: %w", err)
	}

	dataKey, err := tree.Metadata.GetDataKey()
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to retrieve data key: %w", err)
	}

	if _, err := tree.Decrypt(dataKey, aes.NewCipher()); err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to decrypt secrets file: %w", err)
	}

	return store, &tree, dataKey, fileInfo.Mode().Perm(), nil
}

func buildSecretsPath(path string) ([]interface{}, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("secret path cannot be empty")
	}

	fullPath := make([]interface{}, 0, len(parts)+1)
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

type sshKeyInfo struct {
	AuthorizedKey string
	PrivateKeyPEM string
	Generated     bool
}

func resolveSecretsInitPaths(opts *SecretsOptions) (*secretsInitPaths, error) {
	secretsFile := opts.SecretsFile
	if secretsFile == "" {
		secretsFile = "secrets.sops.yaml"
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

func ensureSSHKeyPair(fs afero.Fs, keyFile string, publicKeyFile string) (*sshKeyInfo, error) {
	privateExists, err := afero.Exists(fs, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check key file: %w", err)
	}
	publicExists, err := afero.Exists(fs, publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check public key file: %w", err)
	}

	comment := filepath.Base(keyFile)
	var authorizedKey string
	var privateKeyPEM string

	if privateExists {
		privateKeyBytes, readErr := afero.ReadFile(fs, keyFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read key file: %w", readErr)
		}
		privateKeyPEM = strings.TrimRight(string(privateKeyBytes), "\n")

		if publicExists {
			publicKeyBytes, readErr := afero.ReadFile(fs, publicKeyFile)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read public key file: %w", readErr)
			}
			parsedKey, parsedComment, _, _, parseErr := ssh.ParseAuthorizedKey(publicKeyBytes)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse public key file: %w", parseErr)
			}
			if parsedComment != "" {
				comment = parsedComment
			}
			authorizedKey = marshalAuthorizedKey(parsedKey, comment)
			return &sshKeyInfo{AuthorizedKey: authorizedKey, PrivateKeyPEM: privateKeyPEM}, nil
		}

		rawPrivateKey, parseErr := ssh.ParseRawPrivateKey(privateKeyBytes)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse private key file %s: %w", keyFile, parseErr)
		}

		sshPublicKey, convErr := sshPublicKeyFromPrivateKey(rawPrivateKey)
		if convErr != nil {
			return nil, convErr
		}

		authorizedKey = marshalAuthorizedKey(sshPublicKey, comment)
		if writeErr := writePublicKeyFile(fs, publicKeyFile, authorizedKey); writeErr != nil {
			return nil, writeErr
		}

		return &sshKeyInfo{AuthorizedKey: authorizedKey, PrivateKeyPEM: privateKeyPEM}, nil
	}

	if mkdirErr := fs.MkdirAll(filepath.Dir(keyFile), 0o700); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", mkdirErr)
	}

	publicKey, privateKey, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key pair: %w", genErr)
	}

	sshPublicKey, convErr := ssh.NewPublicKey(publicKey)
	if convErr != nil {
		return nil, fmt.Errorf("failed to convert public key to SSH format: %w", convErr)
	}

	privateKeyBlock, marshalErr := ssh.MarshalPrivateKey(privateKey, comment)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", marshalErr)
	}
	privateKeyBytes := pem.EncodeToMemory(privateKeyBlock)
	privateKeyPEM = strings.TrimRight(string(privateKeyBytes), "\n")

	if writeErr := afero.WriteFile(fs, keyFile, privateKeyBytes, 0o600); writeErr != nil {
		return nil, fmt.Errorf("failed to write private key file: %w", writeErr)
	}

	authorizedKey = marshalAuthorizedKey(sshPublicKey, comment)
	if writeErr := writePublicKeyFile(fs, publicKeyFile, authorizedKey); writeErr != nil {
		return nil, writeErr
	}

	return &sshKeyInfo{AuthorizedKey: authorizedKey, PrivateKeyPEM: privateKeyPEM, Generated: true}, nil
}

func sshPublicKeyFromPrivateKey(privateKey any) (ssh.PublicKey, error) {
	switch typed := privateKey.(type) {
	case ed25519.PrivateKey:
		publicKey, err := ssh.NewPublicKey(typed.Public())
		if err != nil {
			return nil, fmt.Errorf("failed to convert ed25519 public key to SSH format: %w", err)
		}
		return publicKey, nil
	case *ed25519.PrivateKey:
		publicKey, err := ssh.NewPublicKey(typed.Public())
		if err != nil {
			return nil, fmt.Errorf("failed to convert ed25519 public key to SSH format: %w", err)
		}
		return publicKey, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", privateKey)
	}
}

func writePublicKeyFile(fs afero.Fs, publicKeyFile string, authorizedKey string) error {
	if writeErr := afero.WriteFile(fs, publicKeyFile, []byte(authorizedKey+"\n"), 0o644); writeErr != nil {
		return fmt.Errorf("failed to write public key file: %w", writeErr)
	}
	return nil
}

func marshalAuthorizedKey(publicKey ssh.PublicKey, comment string) string {
	trimmed := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	if comment == "" {
		return trimmed
	}
	return trimmed + " " + comment
}

func renderSOPSConfig(displayPublicKeyPath string, recipient string) string {
	return fmt.Sprintf(`creation_rules:
  - path_regex: .*\.sops\.yaml$
    encrypted_regex: "^data$"
    # This is the %s public key, but you can replace it with your own public key
    age: >-
      %s
stores:
  json:
    indent: 2
  json_binary:
    indent: 2
  yaml:
    indent: 2
`, displayPublicKeyPath, recipient)
}

func renderPlainSecretsFile(displayPublicKeyPath string, displayPrivateKeyPath string, authorizedKey string, privateKeyPEM string) string {
	var builder strings.Builder
	builder.WriteString("# cspell: disable\n")
	builder.WriteString("apiVersion: config.karmafun.dev/v1alpha1\n")
	builder.WriteString("kind: SopsGenerator\n")
	builder.WriteString("metadata:\n")
	builder.WriteString("  name: iknite-secrets\n")
	builder.WriteString("  annotations:\n")
	builder.WriteString("    config.kaweezle.com/local-config: \"true\"\n")
	builder.WriteString("    config.kubernetes.io/function: |\n")
	builder.WriteString("      exec:\n")
	builder.WriteString("        path: karmafun\n")
	builder.WriteString("data:\n")
	builder.WriteString("  secrets:\n")
	builder.WriteString(fmt.Sprintf("    # %s\n", displayPublicKeyPath))
	builder.WriteString(fmt.Sprintf("    public_key: &ed25519_public_key %s\n", authorizedKey))
	builder.WriteString(fmt.Sprintf("    # %s\n", displayPrivateKeyPath))
	builder.WriteString("    private_key: &ed25519_private_key |\n")
	for _, line := range strings.Split(privateKeyPEM, "\n") {
		builder.WriteString("      ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
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

	if err := common.EncryptTree(common.EncryptTreeOpts{Tree: &tree, Cipher: aes.NewCipher(), DataKey: dataKey}); err != nil {
		return nil, fmt.Errorf("failed to encrypt initial secrets file: %w", err)
	}

	encryptedData, err := store.EmitEncryptedFile(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to encode encrypted secrets file: %w", err)
	}

	return encryptedData, nil
}

func expandHomePath(homeDir string, path string) string {
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

func displayPath(homeDir string, path string) string {
	if homeDir == "" {
		return path
	}
	relativeToHome, err := filepath.Rel(homeDir, path)
	if err == nil && relativeToHome != "." && relativeToHome != ".." && !strings.HasPrefix(relativeToHome, ".."+string(filepath.Separator)) {
		return filepath.Join("~", relativeToHome)
	}
	if path == homeDir {
		return "~"
	}
	return path
}
