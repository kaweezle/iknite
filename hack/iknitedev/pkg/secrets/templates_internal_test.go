package secrets

import (
	"bytes"
	"testing"
)

func Test_renderTemplate_internal(t *testing.T) {
	t.Parallel()

	t.Run("renders template with values", func(t *testing.T) {
		t.Parallel()

		// cSpell: disable
		data := &TemplateData{Values: map[string]any{
			"publicKeyPath":  "/tmp/id_ed25519.pub",
			"publicKey":      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey",
			"privateKeyPath": "/tmp/id_ed25519",
			"privateKey":     "-----BEGIN OPENSSH PRIVATE KEY-----\nKEYDATA\n-----END OPENSSH PRIVATE KEY-----",
		}}
		// cSpell: enable

		output, err := renderTemplate(SecretsTemplateName, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Contains([]byte(output), []byte("public_key: &ed25519_public_key")) {
			t.Errorf("expected output to contain public key entry, got: %s", output)
		}
		if !bytes.Contains([]byte(output), []byte("private_key: &ed25519_private_key")) {
			t.Errorf("expected output to contain private key entry, got: %s", output)
		}
	})

	t.Run("returns error for missing template", func(t *testing.T) {
		t.Parallel()

		_, err := renderTemplate("notfound.gotmpl", &TemplateData{})
		if err == nil {
			t.Error("expected error for missing template, got nil")
		}
	})
}
