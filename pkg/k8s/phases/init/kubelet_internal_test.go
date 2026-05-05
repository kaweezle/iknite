// cSpell: words testutil kubeadmapi features
package init

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmScheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/config"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	mockData "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/init"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s/testutil"
)

func createInitConfiguration() (*kubeadmapi.InitConfiguration, error) {
	cfg := &kubeadmapi.InitConfiguration{
		ClusterConfiguration: kubeadmapi.ClusterConfiguration{
			KubernetesVersion: "1.35.3",
			FeatureGates: map[string]bool{
				features.NodeLocalCRISocket: true,
			},
			// ComponentConfigs: kubeadmapi.ComponentConfigMap{},
		},
		NodeRegistration: kubeadmapi.NodeRegistrationOptions{
			CRISocket: "unix:///var/run/cri.sock",
			KubeletExtraArgs: []kubeadmapi.Arg{
				{
					Name:  "node-ip",
					Value: "192.168.99.2",
				},
			},
		},
	}
	kubeadmScheme.Scheme.Default(cfg)
	kubeadmScheme.Scheme.Default(&cfg.ClusterConfiguration)
	err := config.SetInitDynamicDefaults(cfg, true)
	if err != nil {
		return nil, fmt.Errorf("cannot set defaults: %w", err)
	}
	return cfg, nil
}

//nolint:gocritic // Use of interface pointer is intentional.
func configureKubeletStartDataMock(
	t *testing.T,
	m *mockData.MockKubeletStartData,
	dir string,
	createdProcess *host.Process,
) {
	t.Helper()

	cfg, err := createInitConfiguration()
	require.NoError(t, err)
	m.EXPECT().Cfg().Return(cfg).Once()
	m.EXPECT().KubeletDir().Return(dir).Once()
	m.EXPECT().PatchesDir().Return(dir).Once()
	out := &bytes.Buffer{}
	m.EXPECT().OutputWriter().Return(out).Once()

	m.EXPECT().DryRun().Return(false).Once()
	m.EXPECT().Context().Return(t.Context()).Once()
	mh := mockHost.NewMockHost(t)
	m.EXPECT().Host().Return(mh).Once()
	if createdProcess == nil {
		mh.EXPECT().ReadFile("/run/kubelet.pid").Return(nil, errors.New("file error")).Maybe()
		mh.EXPECT().ReadFile("/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Maybe()
	} else {
		testutil.MockKubeletStartHost(t, mh)
		m.EXPECT().SetKubeletProcess(mock.Anything).RunAndReturn(func(process host.Process) {
			*createdProcess = process
		}).Once()
	}
}

func TestRunKubeletStart(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	m := mockData.NewMockKubeletStartData(t)
	var createdProcess host.Process
	dir := t.TempDir()
	configureKubeletStartDataMock(t, m, dir, &createdProcess)
	err := runKubeletStart(m)
	req.NoError(err)

	// Check that the files have been created
	kubeletEnvFilePath := filepath.Join(dir, "kubeadm-flags.env")
	kubeletConfigFilePath := filepath.Join(dir, "config.yaml")
	req.FileExists(kubeletEnvFilePath)
	req.FileExists(kubeletConfigFilePath)
	content, err := os.ReadFile(kubeletEnvFilePath) //nolint:gosec // This is a test.
	req.NoError(err)
	req.Contains(strings.TrimSpace(string(content)), strings.TrimSpace(testutil.KubeAdmFlagsFileContent))
	req.NotNil(createdProcess)
}

func TestRunKubeletStart_Errors(t *testing.T) {
	t.Parallel()
	t.Run("Configuration write error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		m := mockData.NewMockKubeletStartData(t)
		dir := "/dev/null"
		m.EXPECT().KubeletDir().Return(dir).Once()
		cfg, err := createInitConfiguration()
		require.NoError(t, err)
		m.EXPECT().Cfg().Return(cfg).Once()

		err = runKubeletStart(m)
		req.Error(err)
		req.Contains(err.Error(), "error writing a dynamic environment file for the kubelet")
	})
	t.Run("Kubelet fun fails", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		m := mockData.NewMockKubeletStartData(t)
		dir := t.TempDir()
		configureKubeletStartDataMock(t, m, dir, nil)
		err := runKubeletStart(m)
		req.Error(err)

		req.Contains(err.Error(), "failed to start kubelet")
	})
}

func TestNewKubeletStartPhase(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	m := mockData.NewMockKubeletStartData(t)
	var createdProcess host.Process
	dir := t.TempDir()
	configureKubeletStartDataMock(t, m, dir, &createdProcess)

	phase := NewKubeletStartPhase()
	req.Equal("kubelet-start", phase.Name)
	req.NotNil(phase.Run)

	err := phase.Run("bad-data")
	req.Error(err)
	req.Contains(err.Error(), "kubelet-start phase invoked with an invalid data struct")

	err = phase.Run(m)
	req.NoError(err)
}
