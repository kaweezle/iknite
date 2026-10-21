package testutil

import (
	"fmt"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	sopsage "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/config"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/kaweezle/iknite/pkg/host"
)

func GetValue(tree *sops.Tree, path string) (string, error) {
	parts := strings.Split(path, ".")
	fullPath := make([]any, 0, len(parts)+1)
	fullPath = append(fullPath, "data")
	for _, part := range parts {
		fullPath = append(fullPath, strings.TrimSpace(part))
	}

	if len(tree.Branches) == 0 { // nocov
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
		if marshalErr != nil { // nocov
			return "", fmt.Errorf("failed to marshal value at %q: %w", path, marshalErr)
		}
		return strings.TrimRight(string(yamlData), "\n"), nil
	}
}

func DecryptDataWithFormat(encrypted []byte, format formats.Format, key string) (*sops.Tree, error) {
	store := common.StoreForFormat(format, config.NewStoresConfig())
	tree, err := store.LoadEncryptedFile(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to parse encrypted secrets: %w", err)
	}
	masterKey := tree.Metadata.KeyGroups[0][0]
	var dataKey []byte
	// convert to age.MasterKey to use the Decrypt method
	ageMasterKey, ok := masterKey.(*sopsage.MasterKey)
	if !ok {
		return nil, fmt.Errorf("expected age.MasterKey, got %T", masterKey)
	}
	var ids sopsage.ParsedIdentities
	ids, err = age.ParseIdentities(strings.NewReader(key))
	if err != nil {
		// try ssh
		id, err2 := agessh.ParseIdentity([]byte(key))
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse SSH identity after age parse failure: %w %w", err, err2)
		}
		ids = []age.Identity{id}
	}
	ids.ApplyToMasterKey(ageMasterKey)
	dataKey, err = ageMasterKey.Decrypt()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data key with SSH identity: %w", err)
	}

	tree.Metadata.DataKey = dataKey

	if _, err := tree.Decrypt(dataKey, aes.NewCipher()); err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets file: %w", err)
	}

	return &tree, nil
}

func AssertSecretValue(t *testing.T, fs host.FileSystem, key, secretsPath, path, want string, contains bool) {
	t.Helper()
	req := require.New(t)

	encrypted, err := fs.ReadFile(secretsPath)
	req.NoError(err, "failed to read secrets file at %s", secretsPath)

	cleartext, err := DecryptDataWithFormat(encrypted, formats.Yaml, key)
	req.NoError(err, "failed to decrypt secrets file at %s", secretsPath)

	AssertSecretValueFromCleartext(t, cleartext, path, want, contains)
}

func AssertSecretValueFromCleartext(t *testing.T, tree *sops.Tree, path, want string, contains bool) {
	t.Helper()
	req := require.New(t)

	value, err := GetValue(tree, path)
	req.NoError(err, "expected to find value at path %s", path)
	req.NotNil(value, "expected to find value at path %s", path)
	if contains {
		req.Contains(value, want, "expected value at path %s to contain %q", path, want)
	} else {
		req.Equal(want, value, "expected value at path %s to equal %q", path, want)
	}
}

func AssertSecretPathMissing(t *testing.T, fs host.FileSystem, key, secretsPath, path string) {
	t.Helper()

	encrypted, err := fs.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("failed to read secrets file: %v", err)
	}

	tree, err := DecryptDataWithFormat(encrypted, formats.Yaml, key)
	if err != nil {
		t.Fatalf("failed to decrypt secrets file: %v", err)
	}

	value, err := GetValue(tree, path)
	if err == nil {
		t.Fatalf("expected path %s to be missing, but found value: %v", path, value)
	}
}
