package init_test

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	initPhases "github.com/kaweezle/iknite/pkg/k8s/phases/init"
)

func TestCreateKineConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wr      io.Writer
		config  *v1alpha1.IkniteClusterSpec
		name    string
		wantErr bool
	}{
		{
			name:    "nil writer returns error",
			wr:      nil,
			config:  &v1alpha1.IkniteClusterSpec{},
			wantErr: true,
		},
		{
			name: "valid config produces output",
			wr:   new(bytes.Buffer),
			config: &v1alpha1.IkniteClusterSpec{
				Ip: net.ParseIP("192.168.99.2"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotErr := initPhases.CreateKineConfiguration(tt.wr, tt.config)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateKineConfiguration() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatalf("CreateKineConfiguration() succeeded unexpectedly: %s", tt.name)
			}

			outputBuf, ok := tt.wr.(*bytes.Buffer)
			if !ok {
				t.Fatal("writer is not a bytes.Buffer")
			}
			outputStr := outputBuf.String()

			expectedImage := ikniteConfig.GetKineImage()
			if !strings.Contains(outputStr, expectedImage) {
				t.Errorf("output does not contain expected image %s", expectedImage)
			}

			if tt.config.Ip != nil {
				if !strings.Contains(outputStr, tt.config.Ip.String()) {
					t.Errorf("output does not contain expected IP %s", tt.config.Ip.String())
				}
			}
		})
	}
}

func TestCreateKineConfiguration_ContainsKineFlags(t *testing.T) {
	t.Parallel()
	buf := new(bytes.Buffer)
	cfg := &v1alpha1.IkniteClusterSpec{
		Ip: net.ParseIP("10.0.0.1"),
	}
	v1alpha1.SetDefaults_IkniteClusterSpec(cfg)

	if err := initPhases.CreateKineConfiguration(buf, cfg); err != nil {
		t.Fatalf("CreateKineConfiguration() failed: %v", err)
	}

	output := buf.String()
	checks := []string{
		"--endpoint=sqlite:///var/lib/kine/kine.db",
		"--trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt",
		"--server-cert-file=/etc/kubernetes/pki/etcd/server.crt",
		"--server-key-file=/etc/kubernetes/pki/etcd/server.key",
		"--listen-address=0.0.0.0:2379",
		"--metrics-bind-address=:2381",
		"name: kine",
		"component: kine",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing expected string %q\n%s", check, output)
		}
	}
}

func TestWriteKineConfiguration(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	manifestDir := "/etc/kubernetes/manifests"
	if err := fs.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("Failed to create manifest directory: %v", err)
	}

	cfg := &v1alpha1.IkniteClusterSpec{
		Ip: net.ParseIP("192.168.99.2"),
	}

	f, err := initPhases.WriteKineConfiguration(fs, manifestDir, cfg)
	if err != nil {
		t.Fatalf("WriteKineConfiguration() failed: %v", err)
	}

	if f == nil {
		t.Fatal("WriteKineConfiguration() returned nil file")
	}

	content, err := fs.ReadFile(manifestDir + "/kine.yaml")
	if err != nil {
		t.Fatalf("Failed to read kine.yaml: %v", err)
	}

	if !strings.Contains(string(content), ikniteConfig.GetKineImage()) {
		t.Errorf("kine.yaml does not contain expected image %s", ikniteConfig.GetKineImage())
	}
}

func TestNewKineControlPlanePhase(t *testing.T) {
	t.Parallel()
	phase := initPhases.NewKineControlPlanePhase()

	if phase.Name != "kine" {
		t.Errorf("expected phase name %q, got %q", "kine", phase.Name)
	}
	if phase.Run == nil {
		t.Error("expected Run function to be set")
	}
}
