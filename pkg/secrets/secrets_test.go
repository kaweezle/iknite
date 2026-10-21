// cSpell: words getsops sopsage filippo
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
package secrets_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/secrets"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const (
	secretsPath = "/test/secrets.sops.yaml"
)

func TestGetSecret(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	testFs := testutil.NewDummyUserHost()
	req.NoError(testFs.Setenv("SOPS_AGE_KEY", testSecretsAgeKey))

	req.NoError(testFs.WriteFile(secretsPath, []byte(testSecretsEncryptedWithData), 0o600))

	opts := &secrets.Options{Fs: testFs, SecretsFile: secretsPath, Logger: testutil.TestLogger(t)}
	value, err := secrets.GetSecret(opts, "github.api_token")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}

	if value != "ghp-test-api-token" {
		t.Fatalf("unexpected get output: %q", value)
	}
}

func TestGetSecretMissingPath(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	testFs := testutil.NewDummyUserHost()
	req.NoError(testFs.Setenv("SOPS_AGE_KEY", testSecretsAgeKey))
	req.NoError(testFs.WriteFile(secretsPath, []byte(testSecretsEncryptedWithData), 0o600))

	opts := &secrets.Options{Fs: testFs, SecretsFile: secretsPath, Logger: testutil.TestLogger(t)}
	_, err := secrets.GetSecret(opts, "github.missing")
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestSetSecret(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	testFs := testutil.NewDummyUserHost()
	req.NoError(testFs.Setenv("SOPS_AGE_KEY", testSecretsAgeKey))
	req.NoError(testFs.WriteFile(secretsPath, []byte(testSecretsEncryptedWithData), 0o600))

	opts := &secrets.Options{Fs: testFs, SecretsFile: secretsPath, Logger: testutil.TestLogger(t)}
	if err := secrets.SetSecret(opts, "github.api_token", "new-token-value"); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	testutil.AssertSecretValue(t, testFs, testSecretsAgeKey, secretsPath, "github.api_token", "new-token-value", false)
}

func TestRemoveSecret(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	testFs := testutil.NewDummyUserHost()
	req.NoError(testFs.Setenv("SOPS_AGE_KEY", testSecretsAgeKey))
	req.NoError(testFs.WriteFile(secretsPath, []byte(testSecretsEncryptedWithData), 0o600))

	opts := &secrets.Options{Fs: testFs, SecretsFile: secretsPath, Logger: testutil.TestLogger(t)}
	if err := secrets.RemoveSecret(opts, "github.api_token"); err != nil {
		t.Fatalf("RemoveSecret failed: %v", err)
	}

	testutil.AssertSecretPathMissing(t, testFs, testSecretsAgeKey, secretsPath, "github.api_token")
}

func TestRemoveSecretMissingPath(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	testFs := testutil.NewDummyUserHost()
	req.NoError(testFs.Setenv("SOPS_AGE_KEY", testSecretsAgeKey))
	req.NoError(testFs.WriteFile(secretsPath, []byte(testSecretsEncryptedWithData), 0o600))

	opts := &secrets.Options{Fs: testFs, SecretsFile: secretsPath, Logger: testutil.TestLogger(t)}
	err := secrets.RemoveSecret(opts, "github.missing")
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestInitSecrets(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := testutil.NewDummyUserHost()
	tempDir := "base"
	homeDir := filepath.Join(tempDir, "home")
	workspaceDir := filepath.Join(tempDir, "workspace")
	secretsPath := filepath.Join(workspaceDir, secrets.DefaultSecretsFile)
	keyPath := filepath.Join(homeDir, ".ssh", "id_ed25519")

	if err := fs.MkdirAll(homeDir, 0o750); err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	if err := fs.MkdirAll(workspaceDir, 0o750); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	opts := &secrets.Options{Fs: fs, SecretsFile: secretsPath, HomeDir: homeDir}
	result, err := secrets.InitSecrets(opts)
	if err != nil {
		t.Fatalf("InitSecrets failed: %v", err)
	}

	assertFileExists(t, fs, filepath.Join(workspaceDir, ".sops.yaml"))
	assertFileExists(t, fs, secretsPath)
	assertFileExists(t, fs, keyPath)
	assertFileExists(t, fs, keyPath+".pub")

	configBytes, err := fs.ReadFile(filepath.Join(workspaceDir, ".sops.yaml"))
	req.NoError(err, "failed to read .sops.yaml")

	configText := string(configBytes)
	if !strings.Contains(configText, "encrypted_regex: \"^data$\"") {
		t.Fatalf("expected .sops.yaml to contain encrypted_regex, got:\n%s", configText)
	}
	if !strings.Contains(configText, "ssh-ed25519 ") {
		t.Fatalf("expected .sops.yaml to contain ssh-ed25519 recipient, got:\n%s", configText)
	}

	keyBytes, err := fs.ReadFile(keyPath)
	key := string(keyBytes)
	req.NoError(err, "failed to read generated SSH private key")
	testutil.AssertSecretValue(t, fs, key, secretsPath, "keys.main.public_key", "ssh-ed25519 ", true)
	testutil.AssertSecretValue(
		t,
		fs,
		key,
		secretsPath,
		"keys.main.private_key",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		true,
	)

	hasWrote := false
	for _, msg := range result.Messages {
		if strings.Contains(msg, "Wrote ") {
			hasWrote = true
			break
		}
	}
	if !hasWrote {
		t.Fatalf("expected init result to contain 'Wrote ' messages, got: %v", result.Messages)
	}
}

func TestInitSecretsDoesNotOverwriteExistingFiles(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := testutil.NewDummyUserHost()
	tempDir := "base"
	workspaceDir := filepath.Join(tempDir, "workspace")
	secretsPath := filepath.Join(workspaceDir, secrets.DefaultSecretsFile)
	sopsConfigPath := filepath.Join(workspaceDir, ".sops.yaml")

	req.NoError(fs.MkdirAll(workspaceDir, 0o750))
	req.NoError(fs.WriteFile(sopsConfigPath, []byte("existing config\n"), 0o600))
	req.NoError(fs.WriteFile(secretsPath, []byte("existing secrets\n"), 0o600))

	opts := &secrets.Options{Fs: fs, SecretsFile: secretsPath, HomeDir: tempDir}
	result, err := secrets.InitSecrets(opts)
	req.NoError(err, "InitSecrets should not fail when files already exist")

	configBytes, err := fs.ReadFile(sopsConfigPath)
	req.NoError(err, "failed to read .sops.yaml")
	req.Equal("existing config\n", string(configBytes), "expected existing .sops.yaml to be preserved")

	secretBytes, err := fs.ReadFile(secretsPath)
	req.NoError(err, "failed to read secrets.sops.yaml")
	req.Equal("existing secrets\n", string(secretBytes), "expected existing secrets.sops.yaml to be preserved")

	hasAlreadyExists := false
	for _, msg := range result.Messages {
		if strings.Contains(msg, "already exists") {
			hasAlreadyExists = true
			break
		}
	}
	req.True(hasAlreadyExists, "expected init result to mention existing files, got: %v", result.Messages)
}

func TestInitSecretsWithCustomKeyFile(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := testutil.NewDummyUserHost()
	tempDir := "base"
	workspaceDir := fs.JoinPath(tempDir, "workspace")
	secretsPath := fs.JoinPath(workspaceDir, secrets.DefaultSecretsFile)
	keyPath := fs.JoinPath(tempDir, "keys", "custom_ed25519")

	req.NoError(fs.MkdirAll(fs.JoinPath(tempDir, "keys"), 0o750))

	opts := &secrets.Options{Fs: fs, SecretsFile: secretsPath, HomeDir: tempDir, KeyFile: keyPath}
	result, err := secrets.InitSecrets(opts)
	req.NoError(err, "InitSecrets should succeed with custom key file")

	hasCfgTip := false
	for _, msg := range result.Messages {
		if strings.Contains(msg, "SOPS_AGE_SSH_PRIVATE_KEY_FILE=") && strings.Contains(msg, keyPath) {
			hasCfgTip = true
			break
		}
	}
	req.True(hasCfgTip, "expected result to contain SSH key env var guidance, got: %v", result.Messages)
}

func TestOptionsSetDefaults(t *testing.T) {
	t.Parallel()
	t.Run("From nothing", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		opts := &secrets.Options{}
		err := opts.SetDefaults()
		req.NoError(err)
		req.NotEmpty(opts.HomeDir, "HomeDir should be set")
		req.NotEmpty(opts.SecretsFile, "SecretsFile should be set")
		req.NotEmpty(opts.KeyFile, "KeyFile should be set")
	})
}

func TestOptionsSetDefaults_Env(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := testutil.NewDummyUserHost()
	req.NoError(fs.Setenv("SOPS_AGE_SSH_PRIVATE_KEY_FILE", "/env/age/key"))
	opts := &secrets.Options{Fs: fs}
	err := opts.SetDefaults()
	req.NoError(err)
	req.Equal("/env/age/key", opts.KeyFile, "KeyFile should be set from env var")
}

func assertFileExists(t *testing.T, fs host.FileSystem, path string) {
	t.Helper()

	exists, err := fs.Exists(path)
	if err != nil {
		t.Fatalf("failed to check if %s exists: %v", path, err)
	}
	if !exists {
		t.Fatalf("expected %s to exist", path)
	}
}

// cSpell: disable
// Regenerate fixture with:
// sops --config <(echo "creation_rules:\n  - encrypted_regex: ^data\$") -e -a 'age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87' plain.yaml | cat.
//
//nolint:lll // static fixture with long encrypted values
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
