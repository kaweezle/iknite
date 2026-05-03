// cSpell: words testutil
package init_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	mockData "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/init"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	ikniteConfig "github.com/kaweezle/iknite/pkg/config"
	"github.com/kaweezle/iknite/pkg/host"
	initPhases "github.com/kaweezle/iknite/pkg/k8s/phases/init"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestWriteStaticPodManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		opts    *initPhases.PodManifestOptions
		config  *v1alpha1.IkniteClusterSpec
		name    string
		wantErr bool
	}{
		{
			name:    "bad options returns error",
			opts:    &initPhases.PodManifestOptions{},
			config:  &v1alpha1.IkniteClusterSpec{},
			wantErr: true,
		},
		{
			name: "valid kube-vip config produces output",
			opts: &initPhases.PodManifestOptions{
				Name:      "kube-vip",
				ImageFunc: ikniteConfig.GetKubeVipImage,
			},
			config: &v1alpha1.IkniteClusterSpec{
				DomainName: "iknite.local",
				Ip:         net.ParseIP("192.168.99.2"),
			},
			wantErr: false,
		},
		{
			name: "valid kine config produces output",
			opts: &initPhases.PodManifestOptions{
				Name:      "kine",
				ImageFunc: ikniteConfig.GetKineImage,
			},
			config: &v1alpha1.IkniteClusterSpec{
				DomainName: "iknite.local",
				Ip:         net.ParseIP("192.168.99.2"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := host.NewMemMapFS()
			manifestDir := "/manifests"

			file, gotErr := initPhases.WriteStaticPodManifest(fs, manifestDir, tt.config, tt.opts)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("WriteStaticPodManifest() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatalf("WriteStaticPodManifest() succeeded unexpectedly: %s", tt.name)
			}

			// Now check that the image is correctly set in the output
			outputBuf, err := fs.ReadFile(file.Name())
			if err != nil {
				t.Fatalf("Failed to read manifest file: %v", err)
			}
			outputStr := string(outputBuf)
			expectedImage := tt.opts.ImageFunc()
			if !bytes.Contains([]byte(outputStr), []byte(expectedImage)) {
				t.Errorf("Output does not contain expected image %s", expectedImage)
			}
		})
	}
}

func TestWriteStaticPodManifest_ContainsKineFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cfg := &v1alpha1.IkniteClusterSpec{
		Ip: net.ParseIP("10.0.0.1"),
	}
	v1alpha1.SetDefaults_IkniteClusterSpec(cfg)

	fs := host.NewMemMapFS()
	manifestDir := "/manifests"
	opts := &initPhases.PodManifestOptions{
		Name:      "kine",
		ImageFunc: ikniteConfig.GetKineImage,
	}

	file, err := initPhases.WriteStaticPodManifest(fs, manifestDir, cfg, opts)
	req.NoError(err)
	buf, err := fs.ReadFile(file.Name())
	req.NoError(err)

	output := string(buf)
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
		req.Contains(output, check)
	}
}

const (
	manifestsDir = "/manifests"
	kineFile     = manifestsDir + "/kine.yaml"
)

func TestWriteStaticPodManifest_Errors(t *testing.T) {
	t.Parallel()

	clusterConfig := &v1alpha1.IkniteClusterSpec{
		DomainName: "iknite.local",
		Ip:         net.ParseIP("192.168.99.2"),
	}
	opts := &initPhases.PodManifestOptions{
		Name:      "kine",
		ImageFunc: ikniteConfig.GetKineImage,
	}

	t.Run("Cannot create directory", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		fs := mockHost.NewMockFileSystem(t)
		fs.EXPECT().MkdirAll(manifestsDir, os.FileMode(0o755)).Return(fmt.Errorf("permission denied")).Once()

		_, err := initPhases.WriteStaticPodManifest(fs, manifestsDir, clusterConfig, opts)
		req.Error(err)
		req.Contains(err.Error(), "failed to create manifest directory")
	})

	t.Run("Cannot create file", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := mockHost.NewMockFileSystem(t)
		fs.EXPECT().MkdirAll(manifestsDir, os.FileMode(0o755)).Return(nil).Once()
		fs.EXPECT().Create(kineFile).Return(nil, fmt.Errorf("permission denied")).Once()

		_, err := initPhases.WriteStaticPodManifest(fs, manifestsDir, clusterConfig, opts)
		req.Error(err)
		req.Contains(err.Error(), "failed to create kine.yaml")
	})

	t.Run("Unknown manifest", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := mockHost.NewMockFileSystem(t)
		_, err := initPhases.WriteStaticPodManifest(fs, manifestsDir, clusterConfig,
			&initPhases.PodManifestOptions{Name: "unknown"})
		req.Error(err)
		req.Contains(err.Error(), "failed to read unknown manifest template")
	})
}

func TestNewKineControlPlanePhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		builder func() workflow.Phase
		name    string
	}{
		{
			name:    "kine",
			builder: initPhases.NewKineControlPlanePhase,
		},
		{
			name:    "kube-vip",
			builder: initPhases.NewKubeVipControlPlanePhase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			phase := tt.builder()

			req.Equal(tt.name, phase.Name)
			req.NotNil(phase.Run)
			md := mockData.NewMockManifestData(t)
			fs := host.NewMemMapFS()
			h, err := testutil.NewDummyHost(fs, &testutil.DummyHostOptions{})
			req.NoError(err)
			cluster := &v1alpha1.IkniteCluster{
				Spec: v1alpha1.IkniteClusterSpec{
					DomainName: "iknite.local",
					Ip:         net.ParseIP("192.168.99.2"),
				},
			}

			md.EXPECT().ManifestDir().Return(manifestsDir).Once()
			md.EXPECT().Host().Return(h).Once()
			md.EXPECT().IkniteCluster().Return(cluster).Once()

			err = phase.Run("bad data struct")
			req.Error(err)
			err = phase.Run(md)
			req.NoError(err)
			exists, err := fs.Exists(manifestsDir + "/" + tt.name + ".yaml")
			req.NoError(err)
			req.True(exists, "Expected manifest file to be created")
		})
	}
}
