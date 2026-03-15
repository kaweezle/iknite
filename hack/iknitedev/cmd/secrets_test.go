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
package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/kaweezle/iknite/hack/iknitedev/cmd"
)

func TestCreateSecretsCmd(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	secretsCmd := cmd.CreateSecretsCmd(fs, nil)
	if secretsCmd == nil {
		t.Fatal("CreateSecretsCmd returned nil")
	}

	if secretsCmd.Use != "secrets" {
		t.Errorf("expected Use to be secrets, got %q", secretsCmd.Use)
	}

	flag := secretsCmd.PersistentFlags().Lookup("secrets-file")
	if flag == nil {
		t.Fatal("expected --secrets-file flag to exist")
	}
	if flag.Shorthand != "s" {
		t.Errorf("expected --secrets-file shorthand to be s, got %q", flag.Shorthand)
	}
	if flag.DefValue != "secrets.sops.yaml" {
		t.Errorf("expected --secrets-file default to be secrets.sops.yaml, got %q", flag.DefValue)
	}

	if len(secretsCmd.Commands()) != 4 {
		t.Fatalf("expected secrets command to have 4 subcommands, got %d", len(secretsCmd.Commands()))
	}
}

func TestSecretsInitCommand(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	workspaceDir := filepath.Join(tempDir, "workspace")
	secretsPath := filepath.Join(workspaceDir, "secrets.sops.yaml")
	keyPath := filepath.Join(homeDir, ".ssh", "id_ed25519")

	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: fs, SecretsFile: secretsPath, HomeDir: homeDir, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(fs, opts)

	var stdout bytes.Buffer
	secretsCmd.SetOut(&stdout)
	secretsCmd.SetArgs([]string{"init"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets init failed: %v", err)
	}

	assertFileExists(t, fs, filepath.Join(workspaceDir, ".sops.yaml"))
	assertFileExists(t, fs, secretsPath)
	assertFileExists(t, fs, keyPath)
	assertFileExists(t, fs, keyPath+".pub")

	configBytes, err := afero.ReadFile(fs, filepath.Join(workspaceDir, ".sops.yaml"))
	if err != nil {
		t.Fatalf("failed to read .sops.yaml: %v", err)
	}
	configText := string(configBytes)
	if !strings.Contains(configText, "encrypted_regex: \"^data$\"") {
		t.Fatalf("expected .sops.yaml to contain encrypted_regex, got:\n%s", configText)
	}
	if !strings.Contains(configText, "ssh-ed25519 ") {
		t.Fatalf("expected .sops.yaml to contain ssh-ed25519 recipient, got:\n%s", configText)
	}

	t.Setenv("SOPS_AGE_SSH_PRIVATE_KEY_FILE", keyPath)
	assertSecretValueFromOSFile(t, secretsPath, "data.secrets.public_key", "ssh-ed25519 ")
	assertSecretValueFromOSFile(t, secretsPath, "data.secrets.private_key", "-----BEGIN OPENSSH PRIVATE KEY-----")

	output := stdout.String()
	if !strings.Contains(output, "Wrote ") {
		t.Fatalf("expected init output to mention written files, got: %s", output)
	}
}

func TestSecretsInitCommandDoesNotOverwriteExistingFiles(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	workspaceDir := filepath.Join(tempDir, "workspace")
	secretsPath := filepath.Join(workspaceDir, "secrets.sops.yaml")
	sopsConfigPath := filepath.Join(workspaceDir, ".sops.yaml")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}
	if err := os.WriteFile(sopsConfigPath, []byte("existing config\n"), 0o644); err != nil {
		t.Fatalf("failed to seed .sops.yaml: %v", err)
	}
	if err := os.WriteFile(secretsPath, []byte("existing secrets\n"), 0o644); err != nil {
		t.Fatalf("failed to seed secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: fs, SecretsFile: secretsPath, HomeDir: tempDir, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(fs, opts)

	var stdout bytes.Buffer
	secretsCmd.SetOut(&stdout)
	secretsCmd.SetArgs([]string{"init"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets init failed: %v", err)
	}

	configBytes, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		t.Fatalf("failed to read .sops.yaml: %v", err)
	}
	if string(configBytes) != "existing config\n" {
		t.Fatalf("expected existing .sops.yaml to be preserved, got: %s", string(configBytes))
	}

	secretBytes, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("failed to read secrets.sops.yaml: %v", err)
	}
	if string(secretBytes) != "existing secrets\n" {
		t.Fatalf("expected existing secrets.sops.yaml to be preserved, got: %s", string(secretBytes))
	}

	output := stdout.String()
	if !strings.Contains(output, "already exists") {
		t.Fatalf("expected init output to mention existing files, got: %s", output)
	}
}

func TestSecretsInitCommandWithCustomKeyFile(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	workspaceDir := filepath.Join(tempDir, "workspace")
	secretsPath := filepath.Join(workspaceDir, "secrets.sops.yaml")
	keyPath := filepath.Join(tempDir, "keys", "custom_ed25519")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: fs, SecretsFile: secretsPath, HomeDir: tempDir, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(fs, opts)

	var stdout bytes.Buffer
	secretsCmd.SetOut(&stdout)
	secretsCmd.SetArgs([]string{"init", "--key-file", keyPath})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets init with custom key failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "SOPS_AGE_SSH_PRIVATE_KEY_FILE=") || !strings.Contains(output, keyPath) {
		t.Fatalf("expected output to contain SSH key env var guidance, got: %s", output)
	}
}

func TestSecretsGetCommand(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)

	var stdout bytes.Buffer
	secretsCmd.SetOut(&stdout)
	secretsCmd.SetArgs([]string{"-s", secretsPath, "get", "github.api_token"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets get failed: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "ghp-test-api-token" {
		t.Fatalf("unexpected get output: %q", got)
	}
}

func TestSecretsGetCommandMissingPath(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)
	secretsCmd.SetArgs([]string{"--secrets-file", secretsPath, "get", "github.missing"})

	err := secretsCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestSecretsSetCommandWithValueArg(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)
	secretsCmd.SetArgs([]string{"--secrets-file", secretsPath, "set", "github.api_token", "new-token-from-arg"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets set failed: %v", err)
	}

	assertSecretValue(t, testFs, secretsPath, "data.github.api_token", "new-token-from-arg")
}

func TestSecretsSetCommandFromStdin(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("new-token-from-stdin\n")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)
	secretsCmd.SetArgs([]string{"--secrets-file", secretsPath, "set", "github.api_token"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets set from stdin failed: %v", err)
	}

	assertSecretValue(t, testFs, secretsPath, "data.github.api_token", "new-token-from-stdin")
}

func TestSecretsRemoveCommand(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)
	secretsCmd.SetArgs([]string{"--secrets-file", secretsPath, "remove", "github.api_token"})

	if err := secretsCmd.Execute(); err != nil {
		t.Fatalf("secrets remove failed: %v", err)
	}

	assertSecretPathMissing(t, testFs, secretsPath, "data.github.api_token")
}

func TestSecretsRemoveCommandMissingPath(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &cmd.SecretsOptions{Fs: testFs, In: strings.NewReader("")}
	secretsCmd := cmd.CreateSecretsCmd(testFs, opts)
	secretsCmd.SetArgs([]string{"--secrets-file", secretsPath, "remove", "github.missing"})

	err := secretsCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func assertFileExists(t *testing.T, fs afero.Fs, path string) {
	t.Helper()

	exists, err := afero.Exists(fs, path)
	if err != nil {
		t.Fatalf("failed to check if %s exists: %v", path, err)
	}
	if !exists {
		t.Fatalf("expected %s to exist", path)
	}
}

func assertSecretValue(t *testing.T, fs afero.Fs, secretsPath string, path string, want string) {
	t.Helper()

	encrypted, err := afero.ReadFile(fs, secretsPath)
	if err != nil {
		t.Fatalf("failed to read secrets file: %v", err)
	}

	cleartext, err := decrypt.DataWithFormat(encrypted, formats.Yaml)
	if err != nil {
		t.Fatalf("failed to decrypt secrets file: %v", err)
	}

	assertSecretValueFromCleartext(t, encrypted, cleartext, path, want)
}

func assertSecretValueFromOSFile(t *testing.T, secretsPath string, path string, wantContains string) {
	t.Helper()

	encrypted, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("failed to read secrets file: %v", err)
	}

	cleartext, err := decrypt.DataWithFormat(encrypted, formats.Yaml)
	if err != nil {
		t.Fatalf("failed to decrypt secrets file: %v", err)
	}

	assertSecretValueFromCleartextContains(t, cleartext, path, wantContains)
}

func assertSecretValueFromCleartext(t *testing.T, encrypted []byte, cleartext []byte, path string, want string) {
	t.Helper()

	var data map[string]any
	if err := yaml.Unmarshal(cleartext, &data); err != nil {
		t.Fatalf("failed to unmarshal cleartext yaml: %v", err)
	}

	got, err := getMapValue(data, strings.Split(path, "."))
	if err != nil {
		t.Fatalf("failed to read value at %s: %v", path, err)
	}

	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("expected string value at %s, got %T", path, got)
	}

	if gotString != want {
		t.Fatalf("unexpected value at %s: got %q, want %q", path, gotString, want)
	}

	if bytes.Contains(encrypted, []byte(want)) {
		t.Fatalf("encrypted file unexpectedly contains plaintext value %q", want)
	}
}

func assertSecretValueFromCleartextContains(t *testing.T, cleartext []byte, path string, wantContains string) {
	t.Helper()

	var data map[string]any
	if err := yaml.Unmarshal(cleartext, &data); err != nil {
		t.Fatalf("failed to unmarshal cleartext yaml: %v", err)
	}

	got, err := getMapValue(data, strings.Split(path, "."))
	if err != nil {
		t.Fatalf("failed to read value at %s: %v", path, err)
	}

	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("expected string value at %s, got %T", path, got)
	}

	if !strings.Contains(gotString, wantContains) {
		t.Fatalf("expected value at %s to contain %q, got %q", path, wantContains, gotString)
	}
}

func assertSecretPathMissing(t *testing.T, fs afero.Fs, secretsPath string, path string) {
	t.Helper()

	encrypted, err := afero.ReadFile(fs, secretsPath)
	if err != nil {
		t.Fatalf("failed to read secrets file: %v", err)
	}

	cleartext, err := decrypt.DataWithFormat(encrypted, formats.Yaml)
	if err != nil {
		t.Fatalf("failed to decrypt secrets file: %v", err)
	}

	var data map[string]any
	if err := yaml.Unmarshal(cleartext, &data); err != nil {
		t.Fatalf("failed to unmarshal cleartext yaml: %v", err)
	}

	if _, err := getMapValue(data, strings.Split(path, ".")); err == nil {
		t.Fatalf("expected path %s to be missing after remove", path)
	}
}

func getMapValue(root map[string]any, parts []string) (any, error) {
	var current any = root
	for _, part := range parts {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path segment %q does not point to a map", part)
		}
		next, ok := asMap[part]
		if !ok {
			return nil, fmt.Errorf("path segment %q not found", part)
		}
		current = next
	}
	return current, nil
}

// cSpell: disable
// Regenerate fixture with:
// sops --config <(echo "creation_rules:\n  - encrypted_regex: ^data\$") -e -a 'age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87' plain.yaml | cat
const testSecretsEncryptedWithData = `apiVersion: autocloud.config.kaweezle.com/v1alpha1
kind: SopsGenerator
data:
    github:
        api_token: ENC[AES256_GCM,data:WllHPtL7LWTKR0LVMZcxNtS5,iv:oLJUFQbSf8R+FXvkm7medaxW4FqlMYNHHApllpOr/vM=,tag:bYF5W47aaeBBh9XDVIip+g==,type:str]
    nested:
        key: ENC[AES256_GCM,data:k4kNlf0=,iv:syCPyNlw/xnhzFF6yVxCOxki+JXivOnp0aa+s0vmQiA=,tag:+bC/Kl8t3+7YzqKGpV2juA==,type:str]
sops:
    age:
        - recipient: age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87
          enc: |
            -----BEGIN AGE ENCRYPTED FILE-----
            YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSBkanE2dnpSWE5Hdm5oZ0JT
            YW1SRHdnSVNZaHBZb21WTk5qQ0VSRk1tWkdzClpDSk1SNXJpYjVycUFkZW93SjdL
            bHVENi9iQ1kxVzBHa0U1cFdnVk5NM0UKLS0tIG4xZ0pRN0pTaTRUZzJtTjIwSENn
            ci9wVW5veHV5STJBUitJK0l3UU5zRzgKPMUBoMmlJRvlxLzrSolQN/bpw94CgEno
            KdV3LZ4TaDh0LdRux+ot2gjifRrGsDxPvXtEqHrI71MiVNCrxGgtJQ==
            -----END AGE ENCRYPTED FILE-----
    lastmodified: "2026-03-15T10:11:02Z"
    mac: ENC[AES256_GCM,data:0w6gsuLW7i1lmnhQTlkPLKoo+j3f/NMJ4Nvj4eiINTFwrTW/0n0E+5kmTTVxULBnccDKpQRjxvh3vq4t4iVFLzkR10rQyv6u+o6IGtSeQKybcpm8JGd66EinRUbheB02WSBzbCJ4yioWMPcEPEoPIHCjJ+mOIStMBXjuoIPdSm4=,iv:PhpUUNXAFUMlatI5ALRir8/4y9jgumPc1XVutp8zC0U=,tag:OBe04AuIia5WhiwvHpca7A==,type:str]
    encrypted_regex: ^data$
    version: 3.12.1`

const testSecretsAgeKey = `# created: 2026-01-22T10:19:48+01:00
# public key: age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87
AGE-SECRET-KEY-1LLH2GKVMQK0RC4YJWCCEQSTKRQKH2P0R6FJYA97960PS54MVVM2SFESHLQ`

// cSpell: enable
