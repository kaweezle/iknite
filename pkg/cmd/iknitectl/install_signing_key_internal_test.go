package iknitectl

// cSpell: words mockfs kyaml rnode getsops wrapcheck

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockfs "github.com/kaweezle/iknite/mocks/pkg/host"
)

func TestCreateSigningKeyCmd_AssignsFSAndExecutes(t *testing.T) {
	t.Parallel()

	fs := mockfs.NewMockFileSystem(t)
	fs.EXPECT().Exists("/secrets.sops.yaml").Return(false, nil)

	opts := &SigningKeyOptions{KeyName: "data.custom_key"}
	cmd := CreateSigningKeyCmd(fs, opts)
	cmd.SetArgs([]string{"/secrets.sops.yaml", "/dest"})

	err := cmd.Execute()
	if err == nil || err.Error() != "secrets file not found: /secrets.sops.yaml" {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Fs != fs {
		t.Fatal("expected filesystem to be assigned to options")
	}
	if opts.SecretsFile != "/secrets.sops.yaml" {
		t.Fatalf("unexpected secrets file: %s", opts.SecretsFile)
	}
	if opts.DestDir != "/dest" {
		t.Fatalf("unexpected destination dir: %s", opts.DestDir)
	}
}

//nolint:gosec // Test-only false positives and serialized hook overrides.
func TestInstallSigningKey_ErrorPathsStruct(t *testing.T) {
	tests := []struct {
		name            string
		existError      error
		mkdirError      error
		writeError      error
		statError       error
		fixtureFilename string
		wantErr         string
	}{
		{
			name:       "exists error",
			existError: errors.New("stat failed"),
			wantErr:    "failed to check if secrets file exists: stat failed",
		},
		{
			name:            "read file error",
			fixtureFilename: "testdata/do_not_exist.yaml",
			wantErr:         "failed to read secrets file",
		},
		{
			name:            "decrypt error",
			fixtureFilename: "testdata/empty_data_plain.yaml",
			wantErr:         "failed to decrypt secrets file",
		},
		{
			name:            "no yaml documents",
			fixtureFilename: "testdata/empty_encrypted.yaml",
			wantErr:         "data.apk_signing_key not found in secrets file",
		},
		{
			name:            "key not found",
			fixtureFilename: "testdata/empty_data_encrypted.yaml",
			wantErr:         "data.apk_signing_key not found in secrets file",
		},
		{
			name:            "invalid signing key format",
			fixtureFilename: "testdata/data_with_key_invalid_encrypted.yaml",
			wantErr:         "data.apk_signing_key has invalid format in secrets file",
		},
		{
			name:            "name missing",
			fixtureFilename: "testdata/data_with_key_no_name_encrypted.yaml",
			wantErr:         "data.apk_signing_key.name not found or invalid in secrets file",
		},
		{
			name:            "private key missing",
			fixtureFilename: "testdata/data_with_key_no_private_key_encrypted.yaml",
			wantErr:         "data.apk_signing_key.private_key not found or invalid in secrets file",
		},
		{
			name:            "mkdir error",
			fixtureFilename: "testdata/data_with_key_complete_encrypted.yaml",
			mkdirError:      errors.New("mkdir failed"),
			wantErr:         "failed to create destination directory: mkdir failed",
		},
		{
			name:            "write error",
			fixtureFilename: "testdata/data_with_key_complete_encrypted.yaml",
			writeError:      errors.New("write failed"),
			wantErr:         "failed to write signing key file: write failed",
		},
		{
			name:            "stat error",
			fixtureFilename: "testdata/data_with_key_complete_encrypted.yaml",
			statError:       errors.New("stat failed"),
			wantErr:         "failed to stat output file: stat failed",
		},
		{
			name:            "nominal case with complete key",
			fixtureFilename: "testdata/data_with_key_complete_encrypted.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			t.Setenv("SOPS_AGE_KEY", testSecretsAgeKey)

			fs := mockfs.NewMockFileSystem(t)
			fs.EXPECT().Exists("/secrets.sops.yaml").Return(tt.existError == nil, tt.existError).Once()
			if tt.existError == nil {
				fs.EXPECT().ReadFile("/secrets.sops.yaml").RunAndReturn(func(_ string) ([]byte, error) {
					return os.ReadFile(tt.fixtureFilename)
				}).Once()
			}
			fs.EXPECT().MkdirAll("/dest", mock.Anything).Return(tt.mkdirError).Maybe()
			fs.EXPECT().WriteFile("/dest/test-signing-key.rsa", mock.Anything, os.FileMode(0o400)).
				RunAndReturn(func(_ string, data []byte, _ os.FileMode) error {
					if tt.writeError != nil {
						return tt.writeError
					}
					req.Equal("private_key_value", string(data), "unexpected data to be written")
					return nil
				}).Maybe()
			fs.EXPECT().Stat("/dest/test-signing-key.rsa").
				RunAndReturn(func(_ string) (os.FileInfo, error) {
					if tt.statError != nil {
						return nil, tt.statError
					}
					return stubFileInfo{name: "test-signing-key.rsa"}, nil
				}).Maybe()

			err := InstallSigningKey(&SigningKeyOptions{
				Fs:          fs,
				KeyName:     "data.apk_signing_key",
				SecretsFile: "/secrets.sops.yaml",
				DestDir:     "/dest",
			})
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				return
			}
			req.NoError(err)
		})
	}
}

// cSpell: disable
// Regenerate fixture with:
//nolint:lll // long line in test data
// sops --config <(echo "creation_rules:\n  - encrypted_regex: ^data\$") -e -a 'age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87' plain.yaml | cat.
//

const testSecretsAgeKey = `# created: 2026-01-22T10:19:48+01:00
# public key: age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87
AGE-SECRET-KEY-1LLH2GKVMQK0RC4YJWCCEQSTKRQKH2P0R6FJYA97960PS54MVVM2SFESHLQ`

// cSpell: enable
