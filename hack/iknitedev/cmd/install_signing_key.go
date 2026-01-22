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
// cSpell: ignore getsops
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// SigningKeyOptions contains configuration for the signing-key command.
type SigningKeyOptions struct {
	Fs          afero.Fs
	KeyName     string
	SecretsFile string
	DestDir     string
}

// CreateSigningKeyCmd creates the signing-key command with the given filesystem and options.
func CreateSigningKeyCmd(fs afero.Fs, opts *SigningKeyOptions) *cobra.Command {
	if opts == nil {
		opts = &SigningKeyOptions{
			KeyName: "apk_signing_key",
		}
	}
	if opts.Fs == nil {
		opts.Fs = fs
	}

	// Capture the initial keyName for the flag default
	defaultKeyName := opts.KeyName

	cmd := &cobra.Command{
		Use:   "signing-key [secrets-file] [destination-directory]",
		Short: "Install APK signing key from SOPS encrypted secrets file",
		Long: `Extract and install an APK signing key from a SOPS encrypted secrets file.

The command decrypts the secrets file using SOPS, extracts the specified
signing key, and writes it to the specified destination directory with appropriate
permissions (0400).

Example:
  iknitedev install signing-key deploy/iac/iknite/secrets.sops.yaml .
  iknitedev install signing-key --key apk_signing_key deploy/iac/iknite/secrets.sops.yaml /path/to/dest`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			opts.SecretsFile = args[0]
			opts.DestDir = args[1]
			return InstallSigningKey(opts)
		},
	}

	cmd.Flags().StringVar(&opts.KeyName, "key", defaultKeyName,
		"Name of the key to extract from secrets file")

	return cmd
}

func InstallSigningKey(opts *SigningKeyOptions) error {
	// Check if secrets file exists
	exists, err := afero.Exists(opts.Fs, opts.SecretsFile)
	if err != nil {
		return fmt.Errorf("failed to check if secrets file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("secrets file not found: %s", opts.SecretsFile)
	}

	//nolint:forbidigo // Intended use of fmt.Printf
	fmt.Printf("Extracting signing key from %s...\n", opts.SecretsFile)

	// Read the encrypted secrets file from the filesystem
	encryptedData, err := afero.ReadFile(opts.Fs, opts.SecretsFile)
	if err != nil {
		return fmt.Errorf("failed to read secrets file: %w", err)
	}

	// Decrypt the secrets file using SOPS (will auto-detect format)
	cleartext, err := decrypt.DataWithFormat(encryptedData, formats.Yaml)
	if err != nil {
		return fmt.Errorf("failed to decrypt secrets file: %w", err)
	}

	// Parse the decrypted YAML
	var secrets map[string]any
	if parseErr := yaml.Unmarshal(cleartext, &secrets); parseErr != nil {
		return fmt.Errorf("failed to parse decrypted secrets: %w", parseErr)
	}

	// Extract the signing key
	signingKeyData, ok := secrets[opts.KeyName]
	if !ok {
		return fmt.Errorf("%s not found in secrets file", opts.KeyName)
	}

	signingKeyMap, ok := signingKeyData.(map[string]any)
	if !ok {
		return fmt.Errorf("%s has invalid format in secrets file", opts.KeyName)
	}

	// Extract the key name and private key
	name, ok := signingKeyMap["name"].(string)
	if !ok {
		return fmt.Errorf("%s.name not found or invalid in secrets file", opts.KeyName)
	}

	privateKey, ok := signingKeyMap["private_key"].(string)
	if !ok {
		return fmt.Errorf("%s.private_key not found or invalid in secrets file", opts.KeyName)
	}

	// Ensure destination directory exists
	if mkdirErr := opts.Fs.MkdirAll(opts.DestDir, 0o755); mkdirErr != nil {
		return fmt.Errorf("failed to create destination directory: %w", mkdirErr)
	}

	// Determine output file path
	outputFile := filepath.Join(opts.DestDir, name+".rsa")

	// Write the private key to file
	if writeErr := afero.WriteFile(opts.Fs, outputFile, []byte(privateKey), 0o400); writeErr != nil {
		return fmt.Errorf("failed to write signing key file: %w", writeErr)
	}

	//nolint:forbidigo // Intended use of fmt.Printf
	fmt.Printf("Successfully created signing key file: %s\n", outputFile)

	// Verify file permissions
	info, err := opts.Fs.Stat(outputFile)
	if err != nil {
		return fmt.Errorf("failed to stat output file: %w", err)
	}
	//nolint:forbidigo // Intended use of fmt.Printf
	fmt.Printf("File permissions: %04o\n", info.Mode().Perm())

	return nil
}
