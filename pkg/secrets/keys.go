package secrets

import (
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"

	"github.com/kaweezle/iknite/pkg/host"
)

type sshKeyInfo struct {
	AuthorizedKey        string
	PrivateKeyPEM        string
	Generated            bool
	AuthorizedKeyDerived bool
}

func marshalAuthorizedKey(publicKey ssh.PublicKey, comment string) string {
	trimmed := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	if comment == "" {
		return trimmed
	}
	return trimmed + " " + comment
}

func sshPublicKeyFromPrivateKey(privateKey any) (ssh.PublicKey, error) {
	var key *ed25519.PrivateKey
	switch typed := privateKey.(type) {
	case ed25519.PrivateKey:
		key = &typed
	case *ed25519.PrivateKey:
		key = typed
	default:
		return nil, fmt.Errorf("unsupported private key type %T", privateKey)
	}
	publicKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, fmt.Errorf("failed to convert ed25519 public key to SSH format: %w", err)
	}
	return publicKey, nil
}

func sshAuthorizedKeyFromPrivateKey(privateKeyBytes []byte, comment string) (string, error) {
	rawPrivateKey, parseErr := ssh.ParseRawPrivateKey(privateKeyBytes)
	if parseErr != nil {
		return "", fmt.Errorf("failed to parse private key file: %w", parseErr)
	}

	sshPublicKey, convErr := sshPublicKeyFromPrivateKey(rawPrivateKey)
	if convErr != nil {
		return "", convErr
	}

	return marshalAuthorizedKey(sshPublicKey, comment), nil
}

func readAuthorizedKeyFromPublicKeyFile(fs host.FileSystem, publicKeyFile string) (string, error) {
	publicKeyBytes, readErr := fs.ReadFile(publicKeyFile)
	if readErr != nil {
		return "", fmt.Errorf("failed to read public key file: %w", readErr)
	}
	parsedKey, parsedComment, _, _, parseErr := ssh.ParseAuthorizedKey(publicKeyBytes)
	if parseErr != nil {
		return "", fmt.Errorf("failed to parse public key file: %w", parseErr)
	}
	if parsedComment == "" {
		parsedComment = filepath.Base(publicKeyFile)
	}
	return marshalAuthorizedKey(parsedKey, parsedComment), nil
}

func createKeyPair(fs host.FileSystem, keyFile, publicKeyFile, comment string) (*sshKeyInfo, error) {
	result := &sshKeyInfo{Generated: true}

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
	result.PrivateKeyPEM = strings.TrimRight(string(privateKeyBytes), "\n")

	if writeErr := fs.WriteFile(keyFile, privateKeyBytes, 0o600); writeErr != nil {
		return nil, fmt.Errorf("failed to write private key file: %w", writeErr)
	}

	result.AuthorizedKey = marshalAuthorizedKey(sshPublicKey, comment)
	if writeErr := writePublicKeyFile(fs, publicKeyFile, result.AuthorizedKey); writeErr != nil {
		return nil, writeErr
	}

	result.AuthorizedKeyDerived = true
	return result, nil
}

// ensureSSHKeyPair checks for the existence of the SSH key pair and generates it if necessary.
func ensureSSHKeyPair(fs host.FileSystem, keyFile, publicKeyFile string) (*sshKeyInfo, error) {
	privateExists, err := fs.Exists(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check key file: %w", err)
	}
	publicExists, err := fs.Exists(publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check public key file: %w", err)
	}

	if publicExists && !privateExists {
		return nil, fmt.Errorf("public key file %s exists but private key file %s does not", publicKeyFile, keyFile)
	}

	comment := filepath.Base(keyFile)
	result := &sshKeyInfo{Generated: false}

	if publicExists {
		result.AuthorizedKey, err = readAuthorizedKeyFromPublicKeyFile(fs, publicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read authorized key from public key file: %w", err)
		}
	}

	if privateExists {
		privateKeyBytes, readErr := fs.ReadFile(keyFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read key file: %w", readErr)
		}
		result.PrivateKeyPEM = strings.TrimRight(string(privateKeyBytes), "\n")

		if publicExists {
			return result, nil
		}

		result.AuthorizedKey, err = sshAuthorizedKeyFromPrivateKey(privateKeyBytes, comment)
		if err != nil {
			return nil, fmt.Errorf("failed to derive public key from private key: %w", err)
		}

		if writeErr := writePublicKeyFile(fs, publicKeyFile, result.AuthorizedKey); writeErr != nil {
			return nil, writeErr
		}
		result.AuthorizedKeyDerived = true

		return result, nil
	}
	return createKeyPair(fs, keyFile, publicKeyFile, comment)
}

func writePublicKeyFile(fs host.FileSystem, publicKeyFile, authorizedKey string) error {
	if writeErr := fs.WriteFile(publicKeyFile, []byte(authorizedKey+"\n"), 0o644); writeErr != nil {
		return fmt.Errorf("failed to write public key file: %w", writeErr)
	}
	return nil
}
