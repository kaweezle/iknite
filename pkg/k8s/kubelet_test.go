// cSpell: words cgroupfs mockk8s
package k8s_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/spf13/afero/mem"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	mockk8s "github.com/kaweezle/iknite/mocks/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
	"github.com/kaweezle/iknite/pkg/utils"
)

func TestCheckServerRunning(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // Test server, ignoring write errors
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 1, 1, 1*time.Second)
	req.NoError(err)
}

func TestCheckServerRunning_BadResponse(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("bad")) //nolint:errcheck // Test server, ignoring write errors
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 2, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "unexpected response body")
}

func TestCheckServerRunning_NotFound(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, srv.URL, "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "unexpected status code")
}

func TestCheckServerRunning_BadURL(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a simple HTTP server in the background
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Check if the server is running
	badChar := string([]byte{0x7f}) // Creates the ASCII DEL Control Character
	err := k8s.CheckServerRunning(ctx, srv.URL+"/foo"+badChar, "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "failed to create HTTP request")
}

func TestCheckServerRunning_BadServer(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the server is running
	err := k8s.CheckServerRunning(ctx, "http://127.0.0.2/foo", "ok", 1, 1, 1*time.Second)
	req.Error(err)
	req.Contains(err.Error(), "failed to make HTTP request")
}

const (
	//nolint:lll // Ignore long line linter warning
	kubeletEnvFileContent = `
command_args="--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf --cgroup-driver=cgroupfs --config=/var/lib/kubelet/config.yaml"
`
	kubeAdmFlagsFileContent = `
KUBELET_KUBEADM_ARGS="--node-ip=192.168.99.2"
`
)

func TestStarKubelet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepareMock  func(t *testing.T, m *mockHost.MockFileExecutor) error
		expectations func(req *require.Assertions, process host.Process) error
		name         string
		wantErr      string
	}{
		{
			name: "nominal case",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				// Read environment files
				m.On("ReadFile", k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
				m.On("ReadFile", k8s.KubeAdmFlagsFile).Return([]byte(kubeAdmFlagsFileContent), nil).Once()
				// Create log directory
				m.On("MkdirAll", k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
				// Create log file
				m.On("OpenFile", k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
					Return(mem.NewFileHandle(mem.CreateFile("kubelet.log")), nil).
					Once()
				// Start kubelet process
				mockProcess := mockHost.NewMockProcess(t)
				mockProcess.On("Pid").Return(1234).Twice()
				m.On("StartCommand", mock.Anything, mock.Anything).Return(mockProcess, nil).Once()
				pidFilePath := alpine.ServicePidFilePath(k8s.KubeletName)
				m.On("WriteFile", pidFilePath, []byte("1234"), os.FileMode(0o644)).Return(nil).Once()
				return nil
			},
			expectations: func(req *require.Assertions, process host.Process) error {
				req.NotNil(process)
				req.Equal(1234, process.Pid())
				return nil
			},
		},
		{
			name: "kubelet already running",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return([]byte("1234"), nil).Once()
				mockProcess := mockHost.NewMockProcess(t)
				mockProcess.On("Pid").Return(1234).Once()
				mockProcess.On("Signal", mock.Anything).Return(nil).Once()
				m.On("FindProcess", 1234).Return(mockProcess, nil).Once()
				return nil
			},
			expectations: func(req *require.Assertions, process host.Process) error {
				req.NotNil(process)
				req.Equal(1234, process.Pid())
				return nil
			},
		},
		{
			name: "Fail to read kubelet pid file",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return([]byte("abcd"), nil).Once()
				return nil
			},
			wantErr: "failed to convert pid file to integer",
		},
		{
			name: "Fail to read env files",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", k8s.KubeletEnvFile).Return(nil, errors.New("read error")).Once()
				return nil
			},
			wantErr: "failed to read environment file",
		},
		{
			name: "Fail to create log directory",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				// Read environment files
				m.On("ReadFile", k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
				m.On("ReadFile", k8s.KubeAdmFlagsFile).Return([]byte(kubeAdmFlagsFileContent), nil).Once()
				// Create log directory
				m.On("MkdirAll", k8s.KubeletLogDir, os.FileMode(0o755)).Return(errors.New("mkdir error")).Once()
				return nil
			},
			wantErr: "failed to create kubelet log directory",
		},
		{
			name: "Fail to create log file",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				// Read environment files
				m.On("ReadFile", k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
				m.On("ReadFile", k8s.KubeAdmFlagsFile).Return([]byte(kubeAdmFlagsFileContent), nil).Once()
				// Create log directory
				m.On("MkdirAll", k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
				// Create log file
				m.On("OpenFile", k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
					Return(nil, errors.New("open file error")).
					Once()
				return nil
			},
			wantErr: "failed to open kubelet log file",
		},
		{
			name: "Fail to start kubelet process",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				// Read environment files
				m.On("ReadFile", k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
				m.On("ReadFile", k8s.KubeAdmFlagsFile).Return([]byte(kubeAdmFlagsFileContent), nil).Once()
				// Create log directory
				m.On("MkdirAll", k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
				// Create log file
				m.On("OpenFile", k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
					Return(mem.NewFileHandle(mem.CreateFile("kubelet.log")), nil).
					Once()
				m.On("StartCommand", mock.Anything, mock.Anything).Return(nil, errors.New("start command error")).Once()
				return nil
			},
			wantErr: "failed to start subprocess",
		},
		{
			name: "fail to write pid file",
			prepareMock: func(t *testing.T, m *mockHost.MockFileExecutor) error {
				t.Helper()
				// Check if process is already running
				m.On("ReadFile", "/run/kubelet.pid").Return(nil, os.ErrNotExist).Once()
				m.On("ReadFile", "/var/run/supervise-kubelet.pid").Return(nil, os.ErrNotExist).Once()
				// Read environment files
				m.On("ReadFile", k8s.KubeletEnvFile).Return([]byte(kubeletEnvFileContent), nil).Once()
				m.On("ReadFile", k8s.KubeAdmFlagsFile).Return([]byte(kubeAdmFlagsFileContent), nil).Once()
				// Create log directory
				m.On("MkdirAll", k8s.KubeletLogDir, os.FileMode(0o755)).Return(nil).Once()
				// Create log file
				m.On("OpenFile", k8s.KubeletLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0o644)).
					Return(mem.NewFileHandle(mem.CreateFile("kubelet.log")), nil).
					Once()
				// Start kubelet process
				mockProcess := mockHost.NewMockProcess(t)
				mockProcess.On("Pid").Return(1234).Times(3)
				m.On("StartCommand", mock.Anything, mock.Anything).Return(mockProcess, nil).Once()
				pidFilePath := alpine.ServicePidFilePath(k8s.KubeletName)
				m.On("WriteFile", pidFilePath, []byte("1234"), os.FileMode(0o644)).
					Return(errors.New("write file error")).
					Once()
				return nil
			},
			expectations: func(req *require.Assertions, process host.Process) error {
				req.NotNil(process)
				req.Equal(1234, process.Pid())
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			fileExec := mockHost.NewMockFileExecutor(t)
			err := tt.prepareMock(t, fileExec)
			req.NoError(err)

			process, err := k8s.StartKubelet(ctx, fileExec)
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
				err = tt.expectations(req, process)
				req.NoError(err)
			}
		})
	}
}

func exitedProcessState(t *testing.T) *os.ProcessState {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "true")
	req := require.New(t)
	req.NoError(cmd.Run())
	return cmd.ProcessState
}

func TestStartAndConfigureKubelet_NilRuntime(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		nil,
		&utils.KustomizeOptions{},
	)

	req.Error(err)
	req.Contains(err.Error(), "kubelet runtime cannot be nil")
}

func TestStartAndConfigureKubelet_StartKubeletError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	runtime := mockk8s.NewMockKubeletRuntime(t)
	runtime.EXPECT().StartKubelet(mock.Anything).Return(nil, errors.New("start kubelet error")).Once()

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		runtime,
		&utils.KustomizeOptions{},
	)

	req.Error(err)
	req.Contains(err.Error(), "failed to start kubelet")
}

func TestStartAndConfigureKubelet_KubeletHealthError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	runtime := mockk8s.NewMockKubeletRuntime(t)
	process := mockHost.NewMockProcess(t)
	process.On("Wait").Run(func(_ mock.Arguments) {
		time.Sleep(30 * time.Millisecond)
	}).Return(nil).Maybe()
	process.On("State").Return(exitedProcessState(t)).Maybe()
	process.EXPECT().Signal(mock.Anything).Return(errors.New("signal error")).Once()

	runtime.EXPECT().StartKubelet(mock.Anything).Return(process, nil).Once()
	runtime.EXPECT().
		CheckKubeletRunning(mock.Anything, 10, 3, time.Second).
		Return(errors.New("kubelet unhealthy")).
		Once()
	runtime.EXPECT().RemovePidFile().Once()

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		runtime,
		&utils.KustomizeOptions{},
	)

	req.Error(err)
	req.Contains(err.Error(), "error while waiting for kubelet to stop")
	req.Contains(err.Error(), "failed to terminate process")
}

func TestStartAndConfigureKubelet_APIServerHealthError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	runtime := mockk8s.NewMockKubeletRuntime(t)
	process := mockHost.NewMockProcess(t)
	process.On("Wait").Run(func(_ mock.Arguments) {
		time.Sleep(30 * time.Millisecond)
	}).Return(nil).Maybe()
	process.On("State").Return(exitedProcessState(t)).Maybe()
	process.EXPECT().Signal(mock.Anything).Return(errors.New("signal error")).Once()

	runtime.EXPECT().StartKubelet(mock.Anything).Return(process, nil).Once()
	runtime.EXPECT().CheckKubeletRunning(mock.Anything, 10, 3, time.Second).Return(nil).Once()
	runtime.EXPECT().
		CheckClusterRunning(mock.Anything, 30, 2, 10*time.Second).
		Return(errors.New("api unhealthy")).
		Once()
	runtime.EXPECT().RemovePidFile().Once()

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		runtime,
		&utils.KustomizeOptions{},
	)

	req.Error(err)
	req.Contains(err.Error(), "error while waiting for kubelet to stop")
	req.Contains(err.Error(), "failed to terminate process")
}

func TestStartAndConfigureKubelet_KustomizeError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	runtime := mockk8s.NewMockKubeletRuntime(t)

	kustomizeOptions := &utils.KustomizeOptions{Kustomization: "test-kustomization"}
	runtime.EXPECT().StartKubelet(mock.Anything).RunAndReturn(func(ctx context.Context) (host.Process, error) {
		process := host.NewDummyProcess(ctx, &host.DummyProcessOptions{
			Cmd: "kubelet",
			Pid: 1234,
		})
		process.Start(10 * time.Second) //nolint:errcheck // Start the process and ignore errors for testing
		return process, nil
	}).Once()
	runtime.EXPECT().CheckKubeletRunning(mock.Anything, 10, 3, time.Second).Return(nil).Once()
	runtime.EXPECT().CheckClusterRunning(mock.Anything, 30, 2, 10*time.Second).Return(nil).Once()
	runtime.EXPECT().
		Kustomize(mock.Anything, kustomizeOptions).
		Return(errors.New("kustomize error")).
		Once()
	runtime.EXPECT().RemovePidFile().Once()

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		runtime,
		kustomizeOptions,
	)

	req.Error(err)
	req.Contains(err.Error(), "error while waiting for kubelet to stop")
}

func TestStartAndConfigureKubelet_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	runtime := mockk8s.NewMockKubeletRuntime(t)
	waitRelease := make(chan struct{})
	process := mockHost.NewMockProcess(t)
	process.EXPECT().Wait().RunAndReturn(func() error {
		<-waitRelease
		return nil
	}).Once()
	process.EXPECT().State().Return(exitedProcessState(t)).Once()

	kustomizeOptions := &utils.KustomizeOptions{Kustomization: "test-kustomization"}
	runtime.EXPECT().StartKubelet(mock.Anything).Return(process, nil).Once()
	runtime.EXPECT().CheckKubeletRunning(mock.Anything, 10, 3, time.Second).Return(nil).Once()
	runtime.EXPECT().CheckClusterRunning(mock.Anything, 30, 2, 10*time.Second).Return(nil).Once()
	runtime.EXPECT().Kustomize(mock.Anything, kustomizeOptions).
		Run(func(_ context.Context, _ *utils.KustomizeOptions) {
			close(waitRelease)
		}).
		Return(nil).
		Once()
	runtime.EXPECT().RemovePidFile().Once()

	err := k8s.StartAndConfigureKubelet(
		context.Background(),
		runtime,
		kustomizeOptions,
	)

	req.NoError(err)
}

func TestStartAndConfigureKubelet_ContextCanceled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runtime := mockk8s.NewMockKubeletRuntime(t)
	process := mockHost.NewMockProcess(t)
	process.On("Wait").Run(func(_ mock.Arguments) {
		time.Sleep(30 * time.Millisecond)
	}).Return(nil).Maybe()
	process.On("State").Return(exitedProcessState(t)).Maybe()
	process.EXPECT().Signal(mock.Anything).Return(errors.New("signal error")).Once()

	runtime.EXPECT().StartKubelet(mock.Anything).Return(process, nil).Once()
	runtime.EXPECT().CheckKubeletRunning(mock.Anything, 10, 3, time.Second).Return(nil).Maybe()
	runtime.EXPECT().RemovePidFile().Once()

	err := k8s.StartAndConfigureKubelet(
		ctx,
		runtime,
		&utils.KustomizeOptions{},
	)

	req.Error(err)
	req.Contains(err.Error(), "error while waiting for kubelet to stop")
	req.Contains(err.Error(), "failed to terminate process")
}

func TestStartAndConfigureKubelet_RuntimeMethodSignatures(t *testing.T) {
	t.Parallel()
	var _ k8s.KubeletRuntime = &mockk8s.MockKubeletRuntime{}
}
