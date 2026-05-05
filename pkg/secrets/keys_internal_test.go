package secrets

import (
	"crypto/rand"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"

	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: disable-next-line
const onlyPublicTestKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC3testonlycomment\n"

func mustWriteFile(t *testing.T, fs host.FileSystem, path string, content []byte, perm os.FileMode) {
	t.Helper()
	if err := fs.WriteFile(path, content, perm); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func mustExist(t *testing.T, fs host.FileSystem, path string) {
	t.Helper()
	exists, err := fs.Exists(path)
	if err != nil {
		t.Fatalf("failed to check %s existence: %v", path, err)
	}
	if !exists {
		t.Fatalf("expected %s to exist", path)
	}
}

func Test_ensureSSHKeyPair_internal_generatesNewPair(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	keyFile := "/tmp/test_id_ed25519"
	pubFile := keyFile + ".pub"

	info, err := ensureSSHKeyPair(fs, keyFile, pubFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.PrivateKeyPEM == "" || info.AuthorizedKey == "" {
		t.Error("expected key info to be populated")
	}
	mustExist(t, fs, keyFile)
	mustExist(t, fs, pubFile)
}

func Test_ensureSSHKeyPair_internal_readsExistingPair(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	keyFile := "/tmp/existing_id_ed25519"
	pubFile := keyFile + ".pub"

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	privateKeyBlock, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	privateKeyBytes := pem.EncodeToMemory(privateKeyBlock)
	mustWriteFile(t, fs, keyFile, privateKeyBytes, 0o600)

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to create SSH public key: %v", err)
	}
	pubStr := marshalAuthorizedKey(sshPub, "test")
	mustWriteFile(t, fs, pubFile, []byte(pubStr+"\n"), 0o644)

	info, ensureErr := ensureSSHKeyPair(fs, keyFile, pubFile)
	if ensureErr != nil {
		t.Fatalf("unexpected error: %v", ensureErr)
	}
	if info == nil || !strings.Contains(info.AuthorizedKey, "test") {
		t.Error("expected to read authorized key with comment")
	}
}

func Test_ensureSSHKeyPair_internal_errorsWhenPublicExistsWithoutPrivate(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	pubFile := "/tmp/only.pub"
	mustWriteFile(t, fs, pubFile, []byte(onlyPublicTestKey), 0o644)

	_, err := ensureSSHKeyPair(fs, "/tmp/missing", pubFile)
	if err == nil {
		t.Error("expected error when public exists but private missing")
	}
}

func Test_sshAuthorizedKeyFromPrivateKey_internal(t *testing.T) {
	t.Parallel()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	privateKeyBlock, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(privateKeyBlock)

	key, err := sshAuthorizedKeyFromPrivateKey(pemBytes, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(key, "test") {
		t.Error("expected comment in authorized key")
	}
}

func Test_readAuthorizedKeyFromPublicKeyFile_internal(t *testing.T) {
	t.Parallel()

	fs := host.NewMemMapFS()
	pubFile := "/tmp/test.pub"
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to create SSH public key: %v", err)
	}
	pubStr := marshalAuthorizedKey(sshPub, "test")
	mustWriteFile(t, fs, pubFile, []byte(pubStr+"\n"), 0o644)

	key, readErr := readAuthorizedKeyFromPublicKeyFile(fs, pubFile)
	if readErr != nil {
		t.Fatalf("unexpected error: %v", readErr)
	}
	if !strings.Contains(key, "test") {
		t.Error("expected comment in authorized key")
	}
}
