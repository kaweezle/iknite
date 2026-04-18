// cSpell: words fakefi testdir noresolve
package checkers

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/check"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

// fakefi is a minimal os.FileInfo implementation used in tests.
type fakefi struct{ dir bool }

func (f *fakefi) Name() string       { return "fake" }
func (f *fakefi) Size() int64        { return 0 }
func (f *fakefi) Mode() os.FileMode  { return 0 }
func (f *fakefi) ModTime() time.Time { return time.Time{} }
func (f *fakefi) IsDir() bool        { return f.dir }
func (f *fakefi) Sys() any            { return nil }

func TestDifference(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	diff := difference([]string{"a", "b", "c"}, []string{"b", "x"})
	req.Equal([]string{"a", "c"}, diff)
}

func TestFileTreeDifferenceAndCheck(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := "base"
	fs := host.NewMemMapFS()
	req.NoError(fs.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600))
	req.NoError(fs.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	req.NoError(fs.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0o600))

	expected := []string{"a.txt", filepath.Join("sub", "b.txt")}
	missing, extra, err := FileTreeDifference(fs, dir, expected)
	req.NoError(err)
	req.Empty(missing)
	req.Empty(extra)

	// Extra files: fewer expected than actual.
	missing, extra, err = FileTreeDifference(fs, dir, []string{"a.txt"})
	req.NoError(err)
	req.Empty(missing)
	req.NotEmpty(extra)

	// EvalSymlinks error.
	mockFSSymErr := host.NewMockFileSystem(t)
	mockFSSymErr.On("EvalSymlinks", dir).Return("", errors.New("symlink error"))
	_, _, err = FileTreeDifference(mockFSSymErr, dir, expected)
	req.Error(err)
	req.ErrorContains(err, "failed to evaluate symlinks")

	// Walk returns error directly (without calling walkFn).
	mockFSWalkErr := host.NewMockFileSystem(t)
	mockFSWalkErr.On("EvalSymlinks", dir).Return(dir, nil)
	mockFSWalkErr.On("Walk", dir, mock.Anything).Return(errors.New("walk error"))
	_, _, err = FileTreeDifference(mockFSWalkErr, dir, expected)
	req.Error(err)
	req.ErrorContains(err, "failed to walk file tree")

	// Walk calls the walkFn with a non-nil error (covers the `if err != nil { return err }` inside the callback).
	mockFSWalkFnErr := host.NewMockFileSystem(t)
	mockFSWalkFnErr.On("EvalSymlinks", dir).Return(dir, nil)
	fileErr := errors.New("file error")
	mockFSWalkFnErr.On("Walk", dir, mock.Anything).Run(func(args mock.Arguments) {
		walkFn := args.Get(1).(filepath.WalkFunc) //nolint:errcheck,forcetypeassert // No need
		_ = walkFn(dir+"/file", nil, fileErr)     //nolint:errcheck // Walk does not return an error here
	}).Return(fileErr)
	_, _, err = FileTreeDifference(mockFSWalkFnErr, dir, expected)
	req.Error(err)
	req.ErrorContains(err, "failed to walk file tree")

	// FileTreeCheck via mock host (three checks: match, missing, extra).
	mockHost := host.NewMockHost(t)
	mockProvider := host.NewMockHostProvider(t)
	mockProvider.On("Host").Return(mockHost).Times(3)

	mockHost.On("EvalSymlinks", mock.Anything).Return(dir, nil).Times(3)
	mockHost.On("Walk", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		path := args.Get(0).(string)              //nolint:errcheck,forcetypeassert // No need
		walkFn := args.Get(1).(filepath.WalkFunc) //nolint:errcheck,forcetypeassert // No need
		//nolint:errcheck // WalkFunc does not return an error.
		_ = fs.Walk(
			path,
			walkFn,
		)
	}).Return(nil).Times(3)

	treeCheck := FileTreeCheck("tree", "tree check", dir, expected)
	ok, msg, err := treeCheck.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "All expected")

	badCheck := FileTreeCheck("tree", "tree check", dir, []string{"missing"})
	ok, msg, err = badCheck.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "Missing files")

	// Extra files: actual dir has more files than expected.
	extraCheck := FileTreeCheck("tree", "tree check", dir, []string{"a.txt"})
	ok, msg, err = extraCheck.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "Extra files")

	// FileTreeCheck with invalid check data type.
	invalidCheck := FileTreeCheck("tree", "tree check", dir, expected)
	ok, _, err = invalidCheck.CheckFn(context.Background(), "not-a-host-provider")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	// FileTreeCheck where FileTreeDifference returns an error (EvalSymlinks fails via mock).
	mockErrProvider := host.NewMockHostProvider(t)
	mockErrHost := host.NewMockHost(t)
	mockErrProvider.On("Host").Return(mockErrHost)
	mockErrHost.On("EvalSymlinks", dir).Return("", errors.New("symlink fail"))
	errCheck := FileTreeCheck("tree", "tree check", dir, expected)
	ok, _, err = errCheck.CheckFn(context.Background(), mockErrProvider)
	req.Error(err)
	req.False(ok)
}

func TestKubernetesFileCheckAndSystemFileCheck(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "conf.txt")
	req.NoError(os.WriteFile(file, []byte("ok\n"), 0o600))
	h := host.NewDefaultHost()
	mockProvider := host.NewMockHostProvider(t)
	mockProvider.On("Host").Return(h).Times(3)

	kubeCheck := SimpleFileCheck("kube-file", file)
	ok, msg, err := kubeCheck.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "exists and is a file")

	contentCheck := FileCheck("sys-file", "desc", file, "ok")
	ok, msg, err = contentCheck.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "expected content")

	badeCheck := FileCheck("sys-file", "desc", file, "bad")
	ok, _, err = badeCheck.CheckFn(context.Background(), mockProvider)
	req.Error(err)
	req.False(ok)

	// Invalid check data type for all three check factories.
	ok, _, err = SimpleFileCheck("x", file).CheckFn(context.Background(), "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	ok, _, err = FileCheck("x", "d", file, "ok").CheckFn(context.Background(), "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")
}

func TestCheckWorkloadDataAccessors(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	alpineHost := host.NewDefaultHost()
	waitOptions := utils.NewWaitOptions()
	raw := CreateCheckWorkloadData("10.0.0.1", waitOptions, alpineHost)
	data, ok := raw.(*checkWorkloadData)
	req.True(ok)

	ready := []*v1alpha1.WorkloadState{{Namespace: "ns", Name: "r", Ok: true, Message: "ok"}}
	unready := []*v1alpha1.WorkloadState{{Namespace: "ns", Name: "u", Ok: false, Message: "pending"}}

	data.SetOk(true)
	data.SetWorkloadCount(2)
	data.SetReadyWorkloads(ready)
	data.SetNotReadyWorkloads(unready)
	data.SetIteration(3)
	data.SetOkIterations(2)
	req.Zero(data.Duration())
	data.Start()
	time.Sleep(2 * time.Millisecond)

	req.True(data.IsOk())
	req.Equal(2, data.WorkloadCount())
	req.Equal(ready, data.ReadyWorkloads())
	req.Equal(unready, data.NotReadyWorkloads())
	req.Equal(3, data.Iteration())
	req.Equal(2, data.OkIterations())
	req.Equal("10.0.0.1", data.ApiAdvertiseAddress())
	req.NotNil(data.WaitOptions())
	req.Greater(data.Duration(), time.Duration(0))
	req.NotEmpty(data.ManifestDir())
	req.Equal(alpineHost, data.Host())
}

func TestWorkloadResultPrinters(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	state := &v1alpha1.WorkloadState{Namespace: "kube-system", Name: "coredns", Message: "Ready", Ok: true}
	line := PrettyPrintWorkloadState("  ", state)
	req.Contains(line, "kube-system")
	req.Contains(line, "coredns")

	data := &checkWorkloadData{waitOptions: utils.NewWaitOptions()}
	data.SetWorkloadCount(2)
	data.SetReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "a", Ok: true, Message: "ok"}})
	data.SetNotReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "b", Ok: false, Message: "pending"}})
	data.Start()

	result := &check.CheckResult{
		Check:     &check.Check{Name: "workloads", Description: "workloads"},
		Status:    check.StatusRunning,
		CheckData: data,
	}

	out := CheckWorkloadResultPrinter(result, "", "*")
	req.Contains(out, "workloads")
	req.Contains(out, "ns")

	result.Status = check.StatusSkipped
	out = CheckWorkloadResultPrinter(result, "", "*")
	req.Contains(out, "workloads")

	// elapsed == 0: Start() not called, so Duration() returns 0.
	dataNoStart := &checkWorkloadData{waitOptions: utils.NewWaitOptions()}
	dataNoStart.SetWorkloadCount(1)
	dataNoStart.SetReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "c", Ok: true, Message: "ok"}})
	dataNoStart.SetNotReadyWorkloads(nil)
	resultNoStart := &check.CheckResult{
		Check:     &check.Check{Name: "w", Description: "w"},
		Status:    check.StatusSuccess,
		CheckData: dataNoStart,
	}
	out = CheckWorkloadResultPrinter(resultNoStart, "", "*")
	req.Contains(out, "w")

	// elapsed > 0: set startTime to 10ms ago so Duration() > 0.5ms after rounding.
	dataElapsed := &checkWorkloadData{
		startTime:   time.Now().Add(-10 * time.Millisecond),
		waitOptions: utils.NewWaitOptions(),
	}
	dataElapsed.SetWorkloadCount(1)
	dataElapsed.SetReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "e", Ok: true, Message: "ok"}})
	dataElapsed.SetNotReadyWorkloads(nil)
	resultElapsed := &check.CheckResult{
		Check:     &check.Check{Name: "elapsed", Description: "elapsed"},
		Status:    check.StatusSuccess,
		CheckData: dataElapsed,
	}
	out = CheckWorkloadResultPrinter(resultElapsed, "", "*")
	req.Contains(out, "elapsed")
	req.Contains(out, "s - ") // elapsed printed in message

	// Only unready workloads: covers the case where len(ready)==0 inside len(unready)>0.
	dataUnreadyOnly := &checkWorkloadData{waitOptions: utils.NewWaitOptions()}
	dataUnreadyOnly.SetWorkloadCount(1)
	dataUnreadyOnly.SetReadyWorkloads(nil)
	dataUnreadyOnly.SetNotReadyWorkloads([]*v1alpha1.WorkloadState{{Namespace: "ns", Name: "d", Ok: false}})
	dataUnreadyOnly.Start()
	resultUnready := &check.CheckResult{
		Check:     &check.Check{Name: "u", Description: "u"},
		Status:    check.StatusFailed,
		CheckData: dataUnreadyOnly,
	}
	out = CheckWorkloadResultPrinter(resultUnready, "", "*")
	req.Contains(out, "u")

	fallback := &check.CheckResult{
		Check:     &check.Check{Name: "x", Description: "x"},
		Status:    check.StatusSuccess,
		CheckData: "bad-data",
	}
	req.Contains(CheckWorkloadResultPrinter(fallback, "", "*"), "x")
}

func TestAdditionalCheckerPaths(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// TODO: Use mocks and make a full test of CheckService.
	h := host.NewDefaultHost()

	ok, msg, err := CheckService(h, "ignored", ServiceTypeOpenRC)
	req.Error(err)
	req.Empty(msg)
	req.False(ok)
	req.ErrorContains(err, "service ignored is not running")

	ok, msg, err = CheckService(h, "ignored", ServiceTypePidFile)
	req.Error(err)
	req.Empty(msg)
	req.False(ok)
	req.ErrorContains(err, "service ignored is not running")

	// CheckService: IsServiceStarted returns an error (Exists fails).
	mockErrHost := host.NewMockHost(t)
	mockErrHost.On("Exists", mock.Anything).Return(false, errors.New("disk error"))
	ok, _, err = CheckService(mockErrHost, "svc", ServiceTypeOpenRC)
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "failed to check if service svc is started")

	// CheckService: IsServiceStarted returns true (success path for OpenRC).
	mockOkHost := host.NewMockHost(t)
	mockOkHost.On("Exists", mock.Anything).Return(true, nil)
	ok, msg, err = CheckService(mockOkHost, "svc", ServiceTypeOpenRC)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "svc")

	// CheckService: CheckPidFile returns a non-ErrNotExist error.
	mockPidErrHost := host.NewMockHost(t)
	mockPidErrHost.On("ReadFile", "/run/svc.pid").Return([]byte(nil), errors.New("permission denied"))
	ok, _, err = CheckService(mockPidErrHost, "svc", ServiceTypePidFile)
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckApiServerHealth(time.Millisecond, "invalid")
	req.Error(err)
	req.False(ok)

	// Pass a valid *checkWorkloadData to cover the kubeconfig-load error path.
	ok, _, err = CheckApiServerHealth(time.Millisecond, &checkWorkloadData{})
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckWorkloads(context.Background(), "invalid")
	req.Error(err)
	req.False(ok)

	// Pass a valid CheckWorkloadData to cover the k8s.LoadFromDefault error path.
	validData := &checkWorkloadData{
		waitOptions: utils.NewWaitOptions(),
		alpineHost:  host.NewDefaultHost(),
	}
	ok, _, err = CheckWorkloads(context.Background(), validData)
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckIkniteServerHealth(context.Background(), &utils.WaitOptions{Wait: false, Watch: false})
	req.Error(err)
	req.False(ok)

	ok, _, err = CheckKubeletHealth(time.Millisecond)
	req.Error(err)
	req.False(ok)

	// ServiceCheck with default parents.
	serviceCheck := ServiceCheck("svc", "containerd", ServiceTypeOpenRC)
	req.NotNil(serviceCheck)
	req.Equal("svc", serviceCheck.Name)
	req.Equal([]string{"openrc"}, serviceCheck.DependsOn)

	// ServiceCheck with explicit parents.
	serviceCheck2 := ServiceCheck("svc2", "containerd", ServiceTypeOpenRC, "custom-dep")
	req.Equal([]string{"custom-dep"}, serviceCheck2.DependsOn)

	// ServiceCheck CheckFn with invalid check data type.
	ok, _, err = serviceCheck.CheckFn(context.Background(), "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	// ServiceCheck CheckFn with valid data (service not running).
	mockSvcProvider := host.NewMockHostProvider(t)
	mockSvcHost := host.NewMockHost(t)
	mockSvcProvider.On("Host").Return(mockSvcHost)
	mockSvcHost.On("Exists", mock.Anything).Return(false, nil)
	ok, _, err = serviceCheck.CheckFn(context.Background(), mockSvcProvider)
	req.Error(err)
	req.False(ok)
}

func TestCheckFileAndContent_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(t *testing.T) host.FileSystem
		path            string
		content         string
		wantErrContains string
	}{
		{
			name: "file does not exist",
			setup: func(_ *testing.T) host.FileSystem {
				return host.NewMemMapFS()
			},
			path:            "/nonexistent",
			wantErrContains: "does not exist",
		},
		{
			name: "stat returns non-ErrNotExist error",
			setup: func(t *testing.T) host.FileSystem {
				t.Helper()
				m := host.NewMockFileSystem(t)
				m.On("Stat", "/test").Return((*fakefi)(nil), errors.New("permission denied"))
				return m
			},
			path:            "/test",
			wantErrContains: "error checking",
		},
		{
			name: "path is a directory",
			setup: func(t *testing.T) host.FileSystem {
				t.Helper()
				fs := host.NewMemMapFS()
				req := require.New(t)
				req.NoError(fs.MkdirAll("/testdir", 0o700))
				return fs
			},
			path:            "/testdir",
			wantErrContains: "is a directory",
		},
		{
			name: "readFile error",
			setup: func(t *testing.T) host.FileSystem {
				t.Helper()
				m := host.NewMockFileSystem(t)
				m.On("Stat", "/test").Return(&fakefi{dir: false}, nil)
				m.On("ReadFile", "/test").Return([]byte(nil), errors.New("read error"))
				return m
			},
			path:            "/test",
			content:         "expected",
			wantErrContains: "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := tt.setup(t)
			ok, _, err := checkFileAndContent(fs, tt.path, tt.content)
			req.Error(err)
			req.False(ok)
			req.ErrorContains(err, tt.wantErrContains)
		})
	}
}

func TestCheckOpenRCAndOpenRCCheck(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// OpenRC is started (file exists).
	fsStarted := host.NewMemMapFS()
	req.NoError(fsStarted.MkdirAll(filepath.Dir(constants.SoftLevelPath), 0o755))
	req.NoError(fsStarted.WriteFile(constants.SoftLevelPath, []byte("default"), 0o644))
	ok, msg, err := checkOpenRCStarted(fsStarted)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "OpenRC is started")

	// OpenRC is not started (file does not exist).
	fsEmpty := host.NewMemMapFS()
	ok, msg, err = checkOpenRCStarted(fsEmpty)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "OpenRC is not started")

	// Exists returns an error.
	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", constants.SoftLevelPath).Return(false, errors.New("disk error"))
	ok, _, err = checkOpenRCStarted(mockFS)
	req.Error(err)
	req.False(ok)

	// OpenRCCheck with invalid check data type.
	chk := OpenRCCheck()
	ok, _, err = chk.CheckFn(context.Background(), "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	// OpenRCCheck with valid data (file exists).
	mockHost := host.NewMockHost(t)
	mockProvider := host.NewMockHostProvider(t)
	mockProvider.On("Host").Return(mockHost).Twice()
	mockHost.On("Exists", constants.SoftLevelPath).Return(true, nil).Once()
	ok, msg, err = chk.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "OpenRC is started")

	// OpenRCCheck with valid data (file does not exist).
	mockHost.On("Exists", constants.SoftLevelPath).Return(false, nil).Once()
	ok, msg, err = chk.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "OpenRC is not started")
}

func TestCheckAPIBackendData(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	// Etcd with data file present.
	fsEtcd := host.NewMemMapFS()
	req.NoError(fsEtcd.MkdirAll("/var/lib/etcd/member/snap", 0o755))
	req.NoError(fsEtcd.WriteFile("/var/lib/etcd/member/snap/db", []byte("data"), 0o600))
	ok, msg, err := checkApiBackendData(fsEtcd, constants.EtcdBackendName)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "has data files")

	// Kine with data file present.
	fsKine := host.NewMemMapFS()
	req.NoError(fsKine.MkdirAll("/var/lib/kine", 0o755))
	req.NoError(fsKine.WriteFile("/var/lib/kine/kine.db", []byte("data"), 0o600))
	ok, msg, err = checkApiBackendData(fsKine, constants.KineBackendName)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "has data files")

	// Missing data file.
	fsEmpty := host.NewMemMapFS()
	req.NoError(fsEmpty.MkdirAll("/var/lib/etcd", 0o755))
	ok, msg, err = checkApiBackendData(fsEmpty, constants.EtcdBackendName)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "has no data file")

	// EvalSymlinks error via mock.
	mockFS := host.NewMockFileSystem(t)
	mockFS.On("EvalSymlinks", mock.Anything).Return("", errors.New("symlink error"))
	ok, _, err = checkApiBackendData(mockFS, constants.EtcdBackendName)
	req.Error(err)
	req.False(ok)

	// APIBackendDataCheck with invalid check data type.
	chk := APIBackendDataCheck(constants.EtcdBackendName)
	ok, _, err = chk.CheckFn(context.Background(), "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	// APIBackendDataCheck with valid data.
	mockHost := host.NewMockHost(t)
	mockProvider := host.NewMockHostProvider(t)
	mockProvider.On("Host").Return(mockHost)
	mockHost.On("EvalSymlinks", mock.Anything).Return("/var/lib/etcd", nil)
	mockHost.On("Walk", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		wfn := args.Get(1).(filepath.WalkFunc) //nolint:errcheck,forcetypeassert // No need
		_ = fsEtcd.Walk("/var/lib/etcd", wfn)  //nolint:errcheck // Walk does not return an error here
	}).Return(nil)
	ok, _, err = chk.CheckFn(context.Background(), mockProvider)
	req.NoError(err)
	req.True(ok)
}

func TestCheckDomainName(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	ip := net.ParseIP("192.168.1.1")
	otherIP := net.ParseIP("10.0.0.1")
	ctx := context.Background()

	// Empty domain name always succeeds.
	ok, msg, err := checkDomainName(ctx, nil, "", ip)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "not set")

	mockNH := host.NewMockNetworkHost(t)

	// Domain mapped to the correct IP.
	mockNH.On("IsHostMapped", ctx, ip, "good.local").Return(true, []net.IP{ip}).Once()
	ok, msg, err = checkDomainName(ctx, mockNH, "good.local", ip)
	req.NoError(err)
	req.True(ok)
	req.Contains(msg, "mapped to IP")

	// Domain resolved but to a different IP.
	mockNH.On("IsHostMapped", ctx, ip, "other.local").Return(true, []net.IP{otherIP}).Once()
	ok, msg, err = checkDomainName(ctx, mockNH, "other.local", ip)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "not mapped to IP")

	// Domain not resolved at all.
	mockNH.On("IsHostMapped", ctx, ip, "noresolve.local").Return(false, []net.IP{}).Once()
	ok, msg, err = checkDomainName(ctx, mockNH, "noresolve.local", ip)
	req.NoError(err)
	req.False(ok)
	req.Contains(msg, "not mapped to IP")

	// DomainNameCheck with invalid check data type.
	chk := DomainNameCheck("test.local", ip)
	ok, _, err = chk.CheckFn(ctx, "bad")
	req.Error(err)
	req.False(ok)
	req.ErrorContains(err, "invalid check data type")

	// DomainNameCheck with valid data (mapped correctly).
	mockHost := host.NewMockHost(t)
	mockProvider := host.NewMockHostProvider(t)
	mockProvider.On("Host").Return(mockHost)
	mockHost.On("IsHostMapped", ctx, ip, "test.local").Return(true, []net.IP{ip})
	ok, _, err = chk.CheckFn(ctx, mockProvider)
	req.NoError(err)
	req.True(ok)
}
