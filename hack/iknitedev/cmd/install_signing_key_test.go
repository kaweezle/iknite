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
// cSpell: words tparallel gocognit gosec gocyclo
package cmd_test

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/kaweezle/iknite/hack/iknitedev/cmd"
)

// cSpell: disable
// Test age private key for SOPS decryption.
const testAgeKey = `# created: 2026-01-22T10:19:48+01:00
# public key: age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87
AGE-SECRET-KEY-1LLH2GKVMQK0RC4YJWCCEQSTKRQKH2P0R6FJYA97960PS54MVVM2SFESHLQ`

//nolint:gosec,lll // SOPS encrypted test data with long lines.
const testSecretsEncrypted = `apk_signing_key:
    name: ENC[AES256_GCM,data:C4eRYYgr8VLiyoKsiqOtkA==,iv:DUM/h+9n2HuPOi6iCJ+kG+Ld/OmETRRXyrK0JXKBc7A=,tag:iUF1tpO2h5au2hz5cggGcA==,type:str]
    private_key: ENC[AES256_GCM,data:9KAZdwXH1fBM1bgX7qI6UdRQcC9nF8O/H2wQBBxFhoGtAkGNVEEP8UI/G+gN8dX0352Hj06CKPOp8veksBwCg1OBl9OTNTABRRYTffDTyYFK2g59q6/lU0XaidYQI6LwK/Ca9wNGrIQGUTGoRmO5swioS5hEnar3vNUvAbiS8/JozWcIzfU83Ue6UArMcO6Mp0b/ceB4g6oH+SORyN61E6ycRpZSYBDwjH88r5nMtX936Dzib4YMpCMuGN6LC3eJlZskUVP4t8F58/x1fI0tEQXDrnJI3TpB4V1ap5rtZWNd8TZ4a+Ai/Hji6H+F54Bk+ZjvnpkOGL1tob0+dp3CQhoAfYXWjTpXjQsOnVNLWzCXbHQLn+3B355W8CLMubgqhvn9NLP9mHsCp5gj1WMA+j3d+Ckoofg4jHUmqXBTpG8B7Q==,iv:vzvRfA+gCROrchhmBEbeZiBhTnMRqM5u7UtEkc0o/a4=,tag:dpyLFRfbyC4luMUFWO3rLw==,type:str]
sops:
    age:
        - recipient: age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87
          enc: |
            -----BEGIN AGE ENCRYPTED FILE-----
            YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSBTT2ZnUWN6YU5MWGhLc0c3
            ZXdacVYyY2l4YkczeFViQzF2emhsZmxXSVRRCnVyOVJzdEI1eENqRmp6Uk9PNUdU
            dkpjc1RlUy9UQkgrNXhQKzFVV1dOcjAKLS0tIDYzbHFseVlHUW5ObnJIa0VyWm9K
            emh2dUMyRGVLTzlBSDRyZWtiQytwaDAKPAfB2S2JeuqOZuJ7EH2WGc/s3cIH26Ik
            iZS8LO8mE6HIfLylwLgSlm8VzrBaERuayM4CdzOjbb8NgjyNe3bc8A==
            -----END AGE ENCRYPTED FILE-----
    lastmodified: "2026-01-22T09:22:43Z"
    mac: ENC[AES256_GCM,data:QrON1nDepyLH2r7dBY2HBtxzhmT9HZMC4X1uLh7/6PzN0rhS6K70HF4bWNt3B0oeeCAWElpXSU69oNDADGGUhr+Qlt1r7J795p+IrbdBcDdY4Nxh2rTL40ZmPrzl/DL9F0vVYkfMIimfAg2x/HyuuGgX8gkDa1DfeCMEFLyutwk=,iv:OdPdYUchiuTHKnGktlPYDk1qoOFMyJAelFp/Y5onam8=,tag:mxCIkHFthSkqtZBoGTP4kQ==,type:str]
    unencrypted_suffix: _unencrypted
    version: 3.11.0`

//nolint:gosec // Test data, not real credentials.
const expectedKeyContent = `-----BEGIN RSA PRIVATE KEY-----
MIIBogIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu
KUpRKfFLfRYC9AIKjbJTWit+CqvjWYzvQwECAwEAAQJAIJLixBy2qpFoS4DSmoEm
o3qGy0t6z09AIJtH+5OeRV1be+N4cDYJKffGzDa88vQENZiRm0GRq6a+HPGQMd2k
TQIhAKMSvzIBnni7ot/OSie2TmJLY4SwTQAevXysE2RbFDYdAiEBCUEGq0g9u+w4
-----END RSA PRIVATE KEY-----
`

// cSpell: enable

//nolint:gocognit,gocyclo,tparallel // Test function with multiple sub tests.
func TestInstallSigningKey(t *testing.T) {
	// Note: Cannot use t.Parallel() in parent when child tests use t.Setenv()

	// Create a memory filesystem for testing
	fs := afero.NewMemMapFs()

	t.Run("CreateSigningKeyCmd with nil options", func(t *testing.T) {
		t.Parallel()
		cmdInstance := cmd.CreateSigningKeyCmd(fs, nil)
		if cmdInstance == nil {
			t.Fatal("CreateSigningKeyCmd returned nil")
		}
		if cmdInstance.Use != "signing-key [secrets-file] [destination-directory]" {
			t.Errorf(
				"Expected Use to be 'signing-key [secrets-file] [destination-directory]', got %q",
				cmdInstance.Use,
			)
		}
	})

	t.Run("CreateSigningKeyCmd with custom options", func(t *testing.T) {
		t.Parallel()
		opts := &cmd.SigningKeyOptions{
			KeyName: "custom_key",
			Fs:      fs,
		}
		cmdInstance := cmd.CreateSigningKeyCmd(fs, opts)
		if cmdInstance == nil {
			t.Fatal("CreateSigningKeyCmd returned nil")
		}

		// Check that the flag default was set correctly
		flag := cmdInstance.Flags().Lookup("key")
		if flag == nil {
			t.Fatal("--key flag not found")
		}
		if flag.DefValue != "custom_key" {
			t.Errorf("Expected --key default to be 'custom_key', got %q", flag.DefValue)
		}
	})

	t.Run("InstallSigningKey with missing secrets file", func(t *testing.T) {
		t.Parallel()
		testFs := afero.NewMemMapFs()
		opts := &cmd.SigningKeyOptions{
			KeyName:     "apk_signing_key",
			SecretsFile: "/nonexistent/secrets.yaml",
			DestDir:     "/tmp/dest",
			Fs:          testFs,
		}

		err := cmd.InstallSigningKey(opts)
		if err == nil {
			t.Fatal("Expected error for missing secrets file, got nil")
		}
	})

	t.Run("Filesystem operations use afero", func(t *testing.T) {
		t.Parallel()
		testFs := afero.NewMemMapFs()
		// Create test directory structure
		testDir := "/test/dest"
		err := testFs.MkdirAll(testDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Verify directory was created in memory filesystem
		exists, err := afero.DirExists(testFs, testDir)
		if err != nil {
			t.Fatalf("Failed to check directory existence: %v", err)
		}
		if !exists {
			t.Error("Expected test directory to exist in memory filesystem")
		}

		// Test writing a file
		testFile := filepath.Join(testDir, "test-key.rsa")
		err = afero.WriteFile(testFs, testFile, []byte("test content"), 0o400)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Verify file was written
		content, err := afero.ReadFile(testFs, testFile)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("Expected file content 'test content', got %q", string(content))
		}

		// Verify file permissions
		info, err := testFs.Stat(testFile)
		if err != nil {
			t.Fatalf("Failed to stat test file: %v", err)
		}
		if info.Mode().Perm() != 0o400 {
			t.Errorf("Expected file permissions 0400, got %04o", info.Mode().Perm())
		}
	})

	t.Run("Options struct can be used directly", func(t *testing.T) {
		t.Parallel()
		// This demonstrates how to test the InstallSigningKey function directly
		// without going through the Cobra command, which is useful for testing
		// the business logic in isolation.
		testFs := afero.NewMemMapFs()

		opts := &cmd.SigningKeyOptions{
			KeyName:     "test_key",
			SecretsFile: "/nonexistent.yaml",
			DestDir:     "/output",
			Fs:          testFs,
		}

		// This would fail because the file doesn't exist, but it demonstrates
		// that we can call the function directly with our options
		err := cmd.InstallSigningKey(opts)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}

		// The error message should mention the file
		if err != nil && err.Error() != "secrets file not found: /nonexistent.yaml" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Complete integration test with encrypted data", func(t *testing.T) {
		// Note: Cannot use t.Parallel() with t.Setenv()

		// Set up SOPS_AGE_KEY environment variable for decryption
		t.Setenv("SOPS_AGE_KEY", testAgeKey)

		// Create a memory filesystem
		testFs := afero.NewMemMapFs()

		// Write the encrypted secrets file to the memory filesystem
		secretsPath := "/test/secrets.sops.yaml"
		if err := afero.WriteFile(testFs, secretsPath, []byte(testSecretsEncrypted), 0o644); err != nil {
			t.Fatalf("Failed to write encrypted secrets file: %v", err)
		}

		// Create options for the test
		destDir := "/test/output"
		opts := &cmd.SigningKeyOptions{
			Fs:          testFs,
			KeyName:     "apk_signing_key",
			SecretsFile: secretsPath,
			DestDir:     destDir,
		}

		// Run the signing key installation
		err := cmd.InstallSigningKey(opts)
		if err != nil {
			t.Fatalf("InstallSigningKey failed: %v", err)
		}

		// Verify the output file was created
		outputFile := filepath.Join(destDir, "test-signing-key.rsa")
		exists, err := afero.Exists(testFs, outputFile)
		if err != nil {
			t.Fatalf("Failed to check if output file exists: %v", err)
		}
		if !exists {
			t.Fatal("Expected output file to exist")
		}

		// Read and verify the content
		content, err := afero.ReadFile(testFs, outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if string(content) != expectedKeyContent {
			t.Errorf(
				"Content mismatch.\nExpected:\n%s\n\nGot:\n%s",
				expectedKeyContent,
				string(content),
			)
		}

		// Verify file permissions
		info, err := testFs.Stat(outputFile)
		if err != nil {
			t.Fatalf("Failed to stat output file: %v", err)
		}
		if info.Mode().Perm() != 0o400 {
			t.Errorf("Expected file permissions 0400, got %04o", info.Mode().Perm())
		}
	})
}
