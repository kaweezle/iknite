package testutil

import (
	"os"
	"testing"

	"github.com/spf13/afero/mem"
	"github.com/stretchr/testify/mock"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	//nolint:lll // Ignore long line linter warning
	kubeletEnvFileContent = `
command_args="--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf --cgroup-driver=cgroupfs --config=/var/lib/kubelet/config.yaml"
`
	KubeAdmFlagsFileContent = `
KUBELET_KUBEADM_ARGS="--node-ip=192.168.99.2"
`
)

type mockFileExecutorExpecter interface {
	*mockHost.MockFileExecutor_Expecter | *mockHost.MockHost_Expecter
}

// The only interest here is to keep the code close
//
//nolint:dupl // Golang generics are not like Rust traits or C++. Code needs to be duplicated.
func MockKubeletStartExpect[M mockFileExecutorExpecter](t *testing.T, m M) {
	t.Helper()
	mockProcess := mockHost.NewMockProcess(t)
	mockProcess.On("Pid").Return(1234).Maybe()
	pidFilePath := alpine.ServicePidFilePath(k8s.KubeletName)
	switch expecter := any(m).(type) {
	case *mockHost.MockFileExecutor_Expecter:
		expecter.ReadFile("/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
		expecter.ReadFile("/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
		// Read environment files
		expecter.ReadFile(k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
		expecter.ReadFile(k8s.KubeAdmFlagsFile).Return([]byte(KubeAdmFlagsFileContent), nil).Once()
		// Create log directory
		expecter.MkdirAll(k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
		// Create log file
		expecter.OpenFile(k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
			Return(mem.NewFileHandle(mem.CreateFile("kubelet.log")), nil).
			Once()
		// Start kubelet process
		expecter.StartCommand(mock.Anything, mock.Anything).Return(mockProcess, nil).Once()
		expecter.WriteFile(pidFilePath, []byte("1234"), os.FileMode(0o644)).Return(nil).Once()
	case *mockHost.MockHost_Expecter:
		expecter.ReadFile("/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
		expecter.ReadFile("/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
		// Read environment files
		expecter.ReadFile(k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
		expecter.ReadFile(k8s.KubeAdmFlagsFile).Return([]byte(KubeAdmFlagsFileContent), nil).Once()
		// Create log directory
		expecter.MkdirAll(k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
		// Create log file
		expecter.OpenFile(k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
			Return(mem.NewFileHandle(mem.CreateFile("kubelet.log")), nil).
			Once()
		// Start kubelet process
		expecter.StartCommand(mock.Anything, mock.Anything).Return(mockProcess, nil).Once()
		expecter.WriteFile(pidFilePath, []byte("1234"), os.FileMode(0o644)).Return(nil).Once()
	default:
		t.Fatal("Not a valid type")
	}
}

func MockKubeletStart(t *testing.T, m *mockHost.MockFileExecutor) {
	t.Helper()
	MockKubeletStartExpect(t, m.EXPECT())
}

func MockKubeletStartHost(t *testing.T, m *mockHost.MockHost) {
	t.Helper()
	MockKubeletStartExpect(t, m.EXPECT())
}
