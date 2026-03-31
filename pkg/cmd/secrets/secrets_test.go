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
package secrets_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	secretsCmd "github.com/kaweezle/iknite/pkg/cmd/secrets"
	"github.com/kaweezle/iknite/pkg/secrets"
)

func TestCreateSecretsCmd(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	cmd := secretsCmd.CreateSecretsCmd(fs, nil)
	if cmd == nil {
		t.Fatal("CreateSecretsCmd returned nil")
	}

	if cmd.Use != "secrets" {
		t.Errorf("expected Use to be secrets, got %q", cmd.Use)
	}

	flag := cmd.PersistentFlags().Lookup("secrets-file")
	if flag == nil {
		t.Fatal("expected --secrets-file flag to exist")
	}
	if flag.Shorthand != "s" {
		t.Errorf("expected --secrets-file shorthand to be s, got %q", flag.Shorthand)
	}
	if flag.DefValue != secrets.DefaultSecretsFile {
		t.Errorf("expected --secrets-file default to be %q, got %q", secrets.DefaultSecretsFile, flag.DefValue)
	}

	if len(cmd.Commands()) != 4 {
		t.Fatalf("expected secrets command to have 4 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestSecretsSetCommandFromStdin(t *testing.T) {
	// Cannot use t.Parallel because this test sets process env for SOPS decryption.
	t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

	testFs := afero.NewMemMapFs()
	secretsPath := "/test/secrets.sops.yaml"
	if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncryptedWithData), 0o644); err != nil {
		t.Fatalf("failed to write test secrets file: %v", err)
	}

	opts := &secrets.Options{Fs: testFs}
	cmd := secretsCmd.CreateSecretsCmd(testFs, opts)
	cmd.SetIn(strings.NewReader("new-token-from-stdin\n"))
	cmd.SetArgs([]string{"--secrets-file", secretsPath, "set", "github.api_token"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("secrets set from stdin failed: %v", err)
	}

	assertSecretValue(t, testFs, secretsPath, "data.github.api_token", "new-token-from-stdin")
}

func assertSecretValue(t *testing.T, fs afero.Fs, secretsPath, path, want string) {
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
	if err = yaml.Unmarshal(cleartext, &data); err != nil {
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
// sops --config <(echo "creation_rules:\n  - encrypted_regex: ^data\$") -e -a 'age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87' plain.yaml | cat.
//
//nolint:lll // Test data
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
