// cSpell: words paralleltest wrapcheck testutil
package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const (
	testSecretsFilePath    = "/tmp/test-secrets.sops.yaml"
	testSOPSConfigFilePath = "/tmp/test-.sops.yaml"
	workspaceDir           = "/workspace"
	homeDir                = "/home/alpine"
)

type errorFileSystem struct {
	host.FileSystem
	existsErrs map[string]error
	statErrs   map[string]error
	readErrs   map[string]error
	writeErrs  map[string]error
}

func (f *errorFileSystem) Exists(path string) (bool, error) {
	if err := f.existsErrs[path]; err != nil {
		return false, err
	}
	exists, err := f.FileSystem.Exists(path)
	if err != nil {
		return false, fmt.Errorf("Exists %s: %w", path, err)
	}
	return exists, nil
}

func (f *errorFileSystem) Stat(path string) (os.FileInfo, error) {
	if err := f.statErrs[path]; err != nil {
		return nil, err
	}
	info, err := f.FileSystem.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("Stat %s: %w", path, err)
	}
	return info, nil
}

func (f *errorFileSystem) ReadFile(path string) ([]byte, error) {
	if err := f.readErrs[path]; err != nil {
		return nil, err
	}
	data, err := f.FileSystem.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ReadFile %s: %w", path, err)
	}
	return data, nil
}

func (f *errorFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := f.writeErrs[path]; err != nil {
		return err
	}
	if err := f.FileSystem.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("WriteFile %s: %w", path, err)
	}
	return nil
}

func TestGetSecret_internal(t *testing.T) {
	t.Parallel()
	t.Run("returns YAML for structured values", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(
			t,
			opts,
			recipient,
			[]byte("data:\n  github:\n    scopes:\n      - repo\n      - workflow\n"),
		)

		value, err := GetSecret(opts, "github.scopes")
		req.NoError(err)
		req.Equal("- repo\n- workflow", value)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		_, err := GetSecret(opts, "github..api_token")
		req.Error(err)
		req.Contains(err.Error(), "invalid secret path")
	})

	t.Run("returns load errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		_, err := GetSecret(
			&Options{Fs: host.NewMemMapFS(), SecretsFile: testSecretsFilePath},
			"github.api_token",
		)
		req.Error(err)
		req.Contains(err.Error(), "secrets file not found")
	})

	t.Run("returns not found for missing secrets path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		_, err := GetSecret(opts, "github.missing")
		req.Error(err)
		req.Contains(err.Error(), "secret path \"github.missing\" not found")
	})

	t.Run("wrong key", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(
			t,
			opts,
			recipient,
			[]byte("data:\n  github:\n    scopes:\n      - repo\n      - workflow\n"),
		)

		newOpts, _ := newSecretsFixture(t)
		newKey, err := newOpts.Fs.ReadFile(newOpts.KeyFile)
		req.NoError(err)
		req.NoError(opts.Fs.WriteFile(opts.KeyFile, newKey, 0o600))

		_, err = GetSecret(opts, "github.scopes")
		req.Error(err)
		req.Contains(err.Error(), "failed to decrypt data key with SSH identity")
	})
}

func TestSetSecret_internal(t *testing.T) {
	t.Parallel()
	t.Run("rejects invalid path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		err := SetSecret(opts, "github..api_token", "new-token")
		req.Error(err)
		req.Contains(err.Error(), "invalid secret path")
	})

	t.Run("returns load errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		err := SetSecret(
			&Options{Fs: host.NewMemMapFS(), SecretsFile: testSecretsFilePath},
			"github.api_token",
			"new-token",
		)
		req.Error(err)
		req.Contains(err.Error(), "secrets file not found")
	})

	t.Run("wraps write errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))
		opts.Fs = &errorFileSystem{
			FileSystem: opts.Fs,
			writeErrs:  map[string]error{opts.SecretsFile: errors.New("write failed")},
		}

		err := SetSecret(opts, "github.api_token", "new-token")
		req.Error(err)
		req.Contains(err.Error(), "failed to write secrets file: write failed")
	})
}

func TestRemoveSecret_internal(t *testing.T) {
	t.Parallel()
	t.Run("rejects invalid path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		err := RemoveSecret(opts, "github..api_token")
		req.Error(err)
		req.Contains(err.Error(), "invalid secret path")
	})

	t.Run("returns load errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		err := RemoveSecret(
			&Options{Fs: host.NewMemMapFS(), SecretsFile: testSecretsFilePath},
			"github.api_token",
		)
		req.Error(err)
		req.Contains(err.Error(), "secrets file not found")
	})

	t.Run("returns not found for missing secrets path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		err := RemoveSecret(opts, "github.missing")
		req.Error(err)
		req.Contains(err.Error(), "secret path \"github.missing\" not found")
	})

	t.Run("wraps write errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))
		opts.Fs = &errorFileSystem{
			FileSystem: opts.Fs,
			writeErrs:  map[string]error{opts.SecretsFile: errors.New("write failed")},
		}

		err := RemoveSecret(opts, "github.api_token")
		req.Error(err)
		req.Contains(err.Error(), "failed to write secrets file: write failed")
	})
}

func TestCheckSecretsFilesExists_internal(t *testing.T) {
	t.Parallel()
	t.Run("returns error when sops config existence check fails", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		paths := &secretsInitPaths{secretsFile: testSecretsFilePath, sopsConfigFile: testSOPSConfigFilePath}
		result := &InitResult{}
		err := checkSecretsFilesExists(
			&Options{Fs: &errorFileSystem{
				FileSystem: host.NewMemMapFS(),
				existsErrs: map[string]error{paths.sopsConfigFile: errors.New("stat failed")},
			}},
			paths,
			result,
		)

		req.Error(err)
		req.Contains(err.Error(), "failed to check .sops.yaml: stat failed")
	})

	t.Run("returns error when secrets existence check fails", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		paths := &secretsInitPaths{secretsFile: testSecretsFilePath, sopsConfigFile: testSOPSConfigFilePath}
		result := &InitResult{}
		fs := host.NewMemMapFS()
		req.NoError(fs.WriteFile(paths.sopsConfigFile, []byte("config"), 0o644))

		err := checkSecretsFilesExists(
			&Options{Fs: &errorFileSystem{
				FileSystem: fs,
				existsErrs: map[string]error{paths.secretsFile: errors.New("read failed")},
			}},
			paths,
			result,
		)

		req.Error(err)
		req.Contains(err.Error(), "failed to check secrets file: read failed")
	})

	t.Run("records existing files", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		paths := &secretsInitPaths{secretsFile: testSecretsFilePath, sopsConfigFile: testSOPSConfigFilePath}
		result := &InitResult{}
		fs := host.NewMemMapFS()
		req.NoError(fs.WriteFile(paths.sopsConfigFile, []byte("config"), 0o644))
		req.NoError(fs.WriteFile(paths.secretsFile, []byte("secrets"), 0o644))

		err := checkSecretsFilesExists(&Options{Fs: fs}, paths, result)
		req.NoError(err)
		req.Len(result.Messages, 2)
	})
}

func TestInitSecrets_internal(t *testing.T) {
	t.Parallel()

	t.Run("wraps existence check errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		secretsFile := filepath.Join(workspaceDir, DefaultSecretsFile)
		sopsConfigFile := filepath.Join(workspaceDir, ".sops.yaml")
		opts := &Options{
			Fs: &errorFileSystem{
				FileSystem: fs,
				existsErrs: map[string]error{sopsConfigFile: errors.New("stat failed")},
			},
			SecretsFile: secretsFile,
			HomeDir:     workspaceDir,
		}
		err := opts.SetDefaults()
		req.NoError(err)

		_, err = InitSecrets(opts)

		req.Error(err)
		req.Contains(err.Error(), "failed to check existing secrets files: failed to check .sops.yaml: stat failed")
	})

	t.Run("returns key pair errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
		publicKeyFile := keyFile + ".pub"
		_, err := createKeyPair(fs, keyFile, publicKeyFile, filepath.Base(keyFile))
		req.NoError(err)
		req.NoError(fs.Remove(keyFile))

		_, err = InitSecrets(&Options{
			Fs:          fs,
			SecretsFile: filepath.Join(homeDir, DefaultSecretsFile),
			HomeDir:     homeDir,
			KeyFile:     keyFile,
		})

		req.Error(err)
		req.Contains(err.Error(), "exists but private key file")
	})

	t.Run("reports derived public key", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
		publicKeyFile := keyFile + ".pub"
		_, err := createKeyPair(fs, keyFile, publicKeyFile, filepath.Base(keyFile))
		req.NoError(err)
		req.NoError(fs.Remove(publicKeyFile))

		result, err := InitSecrets(&Options{
			Fs:          fs,
			SecretsFile: filepath.Join(homeDir, DefaultSecretsFile),
			HomeDir:     homeDir,
			KeyFile:     keyFile,
		})

		req.NoError(err)
		req.Contains(strings.Join(result.Messages, "\n"), "Derived public key from existing private key")
	})

	t.Run("uses existing key pair when force is set", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		secretsFile := filepath.Join(workspaceDir, DefaultSecretsFile)
		sopsConfigFile := filepath.Join(workspaceDir, ".sops.yaml")
		keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
		_, err := createKeyPair(fs, keyFile, keyFile+".pub", filepath.Base(keyFile))
		req.NoError(err)
		req.NoError(fs.WriteFile(sopsConfigFile, []byte("old config"), 0o644))
		req.NoError(fs.WriteFile(secretsFile, []byte("old secrets"), 0o644))

		result, err := InitSecrets(&Options{
			Fs:          fs,
			SecretsFile: secretsFile,
			HomeDir:     homeDir,
			KeyFile:     keyFile,
			Force:       true,
		})

		req.NoError(err)
		req.Contains(strings.Join(result.Messages, "\n"), "Using existing SSH key pair")

		configData, readErr := fs.ReadFile(sopsConfigFile)
		req.NoError(readErr)
		req.NotEqual("old config", string(configData))
	})

	t.Run("wraps sops config write errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		baseFS := host.NewMemMapFS()
		secretsFile := filepath.Join(workspaceDir, DefaultSecretsFile)
		sopsConfigFile := filepath.Join(workspaceDir, ".sops.yaml")
		keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
		_, err := createKeyPair(baseFS, keyFile, keyFile+".pub", filepath.Base(keyFile))
		req.NoError(err)

		_, err = InitSecrets(&Options{
			Fs: &errorFileSystem{
				FileSystem: baseFS,
				writeErrs:  map[string]error{sopsConfigFile: errors.New("write failed")},
			},
			SecretsFile: secretsFile,
			HomeDir:     homeDir,
			KeyFile:     keyFile,
			Force:       true,
		})

		req.Error(err)
		req.Equal(fmt.Sprintf("failed to write %s: write failed", sopsConfigFile), err.Error())
	})

	t.Run("wraps secrets file write errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		baseFS := host.NewMemMapFS()
		secretsFile := filepath.Join(workspaceDir, DefaultSecretsFile)
		keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
		_, err := createKeyPair(baseFS, keyFile, keyFile+".pub", filepath.Base(keyFile))
		req.NoError(err)

		_, err = InitSecrets(&Options{
			Fs: &errorFileSystem{
				FileSystem: baseFS,
				writeErrs:  map[string]error{secretsFile: errors.New("write failed")},
			},
			Logger:      testutil.TestLogger(t),
			SecretsFile: secretsFile,
			HomeDir:     homeDir,
			KeyFile:     keyFile,
			Force:       true,
		})

		req.Error(err)
		req.Equal(fmt.Sprintf("failed to write %s: write failed", secretsFile), err.Error())
	})
}

func TestLoadAndDecryptSecrets_internal(t *testing.T) {
	t.Parallel()
	t.Run("wraps exists errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		_, err := loadAndDecryptSecrets(&Options{
			Fs: &errorFileSystem{
				FileSystem: host.NewMemMapFS(),
				existsErrs: map[string]error{testSecretsFilePath: errors.New("stat failed")},
			},
			SecretsFile: testSecretsFilePath,
		})

		req.Error(err)
		req.Contains(err.Error(), "failed to check secrets file: stat failed")
	})

	t.Run("wraps file errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		secretsFile := testSecretsFilePath
		req.NoError(fs.WriteFile(secretsFile, []byte("test"), 0o600))

		_, err := loadAndDecryptSecrets(&Options{
			Fs: &errorFileSystem{
				FileSystem: fs,
				statErrs:   map[string]error{secretsFile: errors.New("stat failed")},
			},
			SecretsFile: secretsFile,
		})

		req.Error(err)
		req.Contains(err.Error(), "failed to stat secrets file: stat failed")

		_, err = loadAndDecryptSecrets(&Options{
			Fs: &errorFileSystem{
				FileSystem: fs,
				readErrs:   map[string]error{secretsFile: errors.New("read failed")},
			},
			SecretsFile: secretsFile,
		})

		req.Error(err)
		req.Contains(err.Error(), "failed to read secrets file: read failed")
	})

	t.Run("wraps parse errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := host.NewMemMapFS()
		secretsFile := testSecretsFilePath
		req.NoError(fs.WriteFile(secretsFile, []byte("not: [valid"), 0o600))

		_, err := loadAndDecryptSecrets(&Options{Fs: fs, SecretsFile: secretsFile})
		req.Error(err)
		req.Contains(err.Error(), "failed to parse encrypted secrets")
	})

	t.Run("wraps decrypt errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		opts, recipient := newSecretsFixture(t)
		writeEncryptedPlaintext(t, opts, recipient, []byte("data:\n  github:\n    api_token: token\n"))

		encryptedData, err := opts.Fs.ReadFile(opts.SecretsFile)
		req.NoError(err)
		re := regexp.MustCompile(`ENC\[AES256_GCM,[^\]]+\]`)
		corrupted := re.ReplaceAllString(
			string(encryptedData),
			"ENC[AES256_GCM,data:invalid,iv:invalid,tag:invalid,type:str]",
		)
		req.NoError(opts.Fs.WriteFile(opts.SecretsFile, []byte(corrupted), 0o600))

		_, err = loadAndDecryptSecrets(opts)
		req.Error(err)
		req.Contains(err.Error(), "failed to decrypt secrets file")
	})
}

func TestParseSSHIdentityFromPrivateKeyFile_WrongKey(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	fs := host.NewMemMapFS()
	keyFile := filepath.Join(homeDir, ".ssh", "id_ed25519")
	req.NoError(fs.WriteFile(keyFile, []byte("bad key"), 0o600))

	_, err := parseSSHIdentityFromPrivateKeyFile(fs, keyFile)
	req.Error(err)
	req.Contains(err.Error(), "failed to parse SSH identity")
}

func TestHelpers_internal_defaultPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	paths, err := resolveSecretsInitPaths(&Options{Fs: host.NewMemMapFS(), HomeDir: "/home/tester"})
	req.NoError(err)
	req.Equal(DefaultSecretsFile, paths.secretsFile)

	req.Equal("~/.ssh/id_ed25519", expandHomePath("", "~/.ssh/id_ed25519"))
	req.Equal("/etc/hosts", displayPath("", "/etc/hosts"))

	_, err = encryptSecretsPlaintext(DefaultSecretsFile, []byte("not: [valid"), "age1invalid")
	req.Error(err)
	req.Contains(err.Error(), "failed to parse plaintext secrets template")

	_, err = encryptSecretsPlaintext(DefaultSecretsFile, []byte(""), "age1invalid")
	req.Error(err)
	req.Contains(err.Error(), "plaintext secrets template produced no YAML documents")

	_, err = encryptSecretsPlaintext(
		DefaultSecretsFile,
		[]byte("data:\n  github:\n    api_token: token\n"),
		"age1invalid",
	)
	req.Error(err)
	req.Contains(err.Error(), "failed to parse recipient \"age1invalid\"")
}

func newSecretsFixture(t *testing.T) (*Options, string) {
	t.Helper()
	req := require.New(t)

	fs := host.NewMemMapFS()
	tempDir, err := os.UserHomeDir() // use real home dir to avoid permission issues with SSH key generation
	req.NoError(err)
	keyFile := filepath.Join(tempDir, ".ssh", "id_ed25519")
	publicKeyFile := keyFile + ".pub"

	keyInfo, err := createKeyPair(fs, keyFile, publicKeyFile, filepath.Base(keyFile))
	req.NoError(err)

	return &Options{
		Fs:          fs,
		Logger:      testutil.TestLogger(t),
		SecretsFile: filepath.Join(tempDir, DefaultSecretsFile),
		HomeDir:     tempDir,
		KeyFile:     keyFile,
	}, keyInfo.AuthorizedKey
}

func writeEncryptedPlaintext(t *testing.T, opts *Options, recipient string, plaintext []byte) {
	t.Helper()
	req := require.New(t)

	encryptedData, err := encryptSecretsPlaintext(opts.SecretsFile, plaintext, recipient)
	req.NoError(err)
	req.NoError(opts.Fs.WriteFile(opts.SecretsFile, encryptedData, 0o600))
}
