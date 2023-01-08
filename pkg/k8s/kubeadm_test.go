package k8s

import (
	"bytes"
	"io"
	"testing"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/lithammer/dedent"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {

	items := append(make([]interface{}, 0), combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}

func (m *MockExecutor) Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error) {
	items := append(make([]interface{}, 0), stdin, combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}

type KubeadmTestSuite struct {
	suite.Suite
	Executor    *MockExecutor
	OldExecutor *utils.Executor
}

func (s *KubeadmTestSuite) SetupTest() {
	s.Executor = &MockExecutor{}
	s.OldExecutor = &utils.Exec
	utils.Exec = s.Executor
}

func (s *KubeadmTestSuite) TeardownTest() {
	s.Executor = &MockExecutor{}
	s.OldExecutor = &utils.Exec
	utils.Exec = s.Executor
}

const WSLKubeadmConfig = `
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: "1.25.0"
networking:
  podSubnet: 10.244.0.0/16
controlPlaneEndpoint: kaweezle.local
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: 192.168.99.2
skipPhases:
  - mark-control-plane
nodeRegistration:
  name: kaweezle.local
  kubeletExtraArgs:
    node-ip: 192.168.99.2
  ignorePreflightErrors:
    - DirAvailable--var-lib-etcd
    - Swap
`

func (s *KubeadmTestSuite) TestCreateKubeadmConfig() {
	assert := s.Require()
	var config = KubeadmConfig{
		Ip:                "192.168.99.2",
		KubernetesVersion: "1.25.0",
		DomainName:        "kaweezle.local",
		CreateIp:          true,
		NetworkInterface:  "eth0",
	}

	out := new(bytes.Buffer)

	assert.NoError(CreateKubeadmConfiguration(out, &config), "Error while creating configuration")
	actual := out.String()
	assert.Equal(WSLKubeadmConfig, actual, "Configurations should be equal")

}

func (s *KubeadmTestSuite) TestCreateKubeadmConfigVM() {
	assert := s.Require()
	expected := dedent.Dedent(`
    apiVersion: kubeadm.k8s.io/v1beta3
    kind: ClusterConfiguration
    kubernetesVersion: "1.25.0"
    networking:
      podSubnet: 10.244.0.0/16
    ---
    apiVersion: kubeadm.k8s.io/v1beta3
    kind: InitConfiguration
    localAPIEndpoint:
      advertiseAddress: 192.168.99.2
    skipPhases:
      - mark-control-plane
    nodeRegistration:
      kubeletExtraArgs:
        node-ip: 192.168.99.2
      ignorePreflightErrors:
        - DirAvailable--var-lib-etcd
        - Swap
    `)
	var config = KubeadmConfig{
		Ip:                "192.168.99.2",
		KubernetesVersion: "1.25.0",
		CreateIp:          false,
		NetworkInterface:  "eth0",
	}

	out := new(bytes.Buffer)

	assert.NoError(CreateKubeadmConfiguration(out, &config), "Error while creating configuration")
	actual := out.String()
	assert.Equal(expected, actual, "Configurations should be equal")

}

func (s *KubeadmTestSuite) TestWriteKubeadmConfiguration() {
	assert := s.Require()
	expected := dedent.Dedent(`
    apiVersion: kubeadm.k8s.io/v1beta3
    kind: ClusterConfiguration
    kubernetesVersion: "1.25.0"
    networking:
      podSubnet: 10.244.0.0/16
    ---
    apiVersion: kubeadm.k8s.io/v1beta3
    kind: InitConfiguration
    localAPIEndpoint:
      advertiseAddress: 192.168.99.2
    skipPhases:
      - mark-control-plane
    nodeRegistration:
      kubeletExtraArgs:
        node-ip: 192.168.99.2
      ignorePreflightErrors:
        - DirAvailable--var-lib-etcd
        - Swap
    `)
	var config = KubeadmConfig{
		Ip:                "192.168.99.2",
		KubernetesVersion: "1.25.0",
		CreateIp:          false,
		NetworkInterface:  "eth0",
	}

	fs := afero.NewMemMapFs()
	afs := &afero.Afero{Fs: fs}
	f, err := WriteKubeadmConfiguration(fs, &config)

	assert.NoError(err)
	assert.True(afs.Exists(f.Name()))

	actual, err := afs.ReadFile(f.Name())
	assert.NoError(err)
	assert.Equal(expected, string(actual), "Written file is not the same")
}

func (s *KubeadmTestSuite) TestRunKubeadmInit() {

	require := s.Require()
	var config = KubeadmConfig{
		Ip:                "192.168.99.2",
		KubernetesVersion: "1.25.0",
		DomainName:        "kaweezle.local",
		CreateIp:          true,
		NetworkInterface:  "eth0",
	}

	fs := afero.NewOsFs()
	afs := &afero.Afero{Fs: fs}

	fileExists := false
	var configContent string
	s.Executor.On("Run", true, "/usr/bin/kubeadm", "init", "--config", mock.Anything).Run(func(args mock.Arguments) {
		lastArg, ok := args[len(args)-1].(string)
		if ok {
			fileExists, _ = afs.Exists(lastArg)
			if fileExists {
				configBytes, err := afs.ReadFile(lastArg)
				if err == nil {
					configContent = string(configBytes)
				} else {
					log.Error("Error while reading", lastArg, err)
				}
			}
		}

	}).Return("ok", nil)

	err := RunKubeadmInit(&config)
	require.NoError(err)
	s.Executor.AssertExpectations(s.T())
	args := s.Executor.Calls[0].Arguments
	lastArg, ok := args[len(args)-1].(string)
	require.True(ok, "Last argument should be a string")
	require.False(afs.Exists(lastArg))

	require.True(fileExists, "Config file should have been created")
	require.Equal(WSLKubeadmConfig, configContent, "Kubeadm configuration is not what expected")
}

func TestKubeadm(t *testing.T) {
	suite.Run(t, new(KubeadmTestSuite))
}
