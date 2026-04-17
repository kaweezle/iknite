package alpine_test

// cSpell: words runlevel runlevels softlevel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

const (
	testSvc         = "myservice"
	startedSvcLink  = "/run/openrc/started/myservice"
	initSvcFile     = "/etc/init.d/myservice"
	runLevelSvcLink = "/etc/runlevels/default/myservice"
	openRCSrcDir    = "/lib/rc/init.d"
	openRCDir       = "/run/openrc"
)

// --- EnsureOpenRC ---

func TestEnsureOpenRC_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("Run", true, "/sbin/openrc", []string{"default"}).Return([]byte("ok"), nil).Once()

	err := alpine.EnsureOpenRC(mockExec, "default")
	req.NoError(err)
}

func TestEnsureOpenRC_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("Run", true, "/sbin/openrc", []string{"default"}).
		Return(nil, errors.New("openrc failed")).Once()

	err := alpine.EnsureOpenRC(mockExec, "default")
	req.Error(err)
	req.Contains(err.Error(), "error while starting openrc")
}

// --- StartOpenRC ---

func TestStartOpenRC_SoftLevelExists(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	// SoftLevelPath already exists → no Run call expected
	mockFE.On("Exists", constants.SoftLevelPath).Return(true, nil).Once()

	err := alpine.StartOpenRC(mockFE)
	req.NoError(err)
}

func TestStartOpenRC_SoftLevelAbsent_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", constants.SoftLevelPath).Return(false, nil).Once()
	mockFE.On("Run", true, "/sbin/openrc", []string{"default"}).Return([]byte("ok"), nil).Once()

	err := alpine.StartOpenRC(mockFE)
	req.NoError(err)
}

func TestStartOpenRC_SoftLevelAbsent_EnsureOpenRCError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", constants.SoftLevelPath).Return(false, nil).Once()
	mockFE.On("Run", true, "/sbin/openrc", []string{"default"}).
		Return(nil, errors.New("openrc failed")).Once()

	err := alpine.StartOpenRC(mockFE)
	req.Error(err)
	req.Contains(err.Error(), "failed to start OpenRC")
}

func TestStartOpenRC_ExistsError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", constants.SoftLevelPath).
		Return(false, errors.New("stat error")).Once()

	err := alpine.StartOpenRC(mockFE)
	req.Error(err)
}

// --- IsServiceStarted ---

func TestIsServiceStarted_True(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	// Create the started-service link as a regular file (simulates existence)
	req.NoError(fs.WriteFile(startedSvcLink, []byte(""), 0o644))

	started, err := alpine.IsServiceStarted(fs, testSvc)
	req.NoError(err)
	req.True(started)
}

func TestIsServiceStarted_False(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()

	started, err := alpine.IsServiceStarted(fs, testSvc)
	req.NoError(err)
	req.False(started)
}

func TestIsServiceStarted_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(false, errors.New("stat error")).Once()

	_, err := alpine.IsServiceStarted(mockFS, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "failed to check if service")
}

// --- ExecuteIfServiceNotStarted ---

func TestExecuteIfServiceNotStarted_NotStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS() // link absent → not started

	executed := false
	err := alpine.ExecuteIfServiceNotStarted(fs, testSvc, func() error {
		executed = true
		return nil
	})
	req.NoError(err)
	req.True(executed)
}

func TestExecuteIfServiceNotStarted_AlreadyStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	req.NoError(fs.WriteFile(startedSvcLink, []byte(""), 0o644))

	executed := false
	err := alpine.ExecuteIfServiceNotStarted(fs, testSvc, func() error {
		executed = true
		return nil
	})
	req.NoError(err)
	req.False(executed)
}

func TestExecuteIfServiceNotStarted_ExistsError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(false, errors.New("stat error")).Once()

	err := alpine.ExecuteIfServiceNotStarted(mockFS, testSvc, func() error { return nil })
	req.Error(err)
	req.Contains(err.Error(), "error while checking if service")
}

// --- ExecuteIfServiceStarted ---

func TestExecuteIfServiceStarted_Started(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	req.NoError(fs.WriteFile(startedSvcLink, []byte(""), 0o644))

	executed := false
	err := alpine.ExecuteIfServiceStarted(fs, testSvc, func() error {
		executed = true
		return nil
	})
	req.NoError(err)
	req.True(executed)
}

func TestExecuteIfServiceStarted_NotStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()

	executed := false
	err := alpine.ExecuteIfServiceStarted(fs, testSvc, func() error {
		executed = true
		return nil
	})
	req.NoError(err)
	req.False(executed)
}

func TestExecuteIfServiceStarted_ExistsError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(false, errors.New("stat error")).Once()

	err := alpine.ExecuteIfServiceStarted(mockFS, testSvc, func() error { return nil })
	req.Error(err)
	req.Contains(err.Error(), "error while checking if service")
}

// --- EnableService ---

func TestEnableService_NotYetEnabled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(false, nil).Once()
	mockFS.On("Symlink", initSvcFile, runLevelSvcLink).Return(nil).Once()

	err := alpine.EnableService(mockFS, testSvc)
	req.NoError(err)
}

func TestEnableService_AlreadyEnabled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(true, nil).Once()
	// Symlink should NOT be called

	err := alpine.EnableService(mockFS, testSvc)
	req.NoError(err)
}

func TestEnableService_SymlinkError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(false, nil).Once()
	mockFS.On("Symlink", initSvcFile, runLevelSvcLink).Return(errors.New("symlink failed")).Once()

	err := alpine.EnableService(mockFS, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "failed to enable service")
}

func TestEnableService_ExistsError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(false, errors.New("stat error")).Once()

	err := alpine.EnableService(mockFS, testSvc)
	req.Error(err)
}

// --- DisableService ---

func TestDisableService_Enabled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(true, nil).Once()
	mockFS.On("Remove", runLevelSvcLink).Return(nil).Once()

	err := alpine.DisableService(mockFS, testSvc)
	req.NoError(err)
}

func TestDisableService_NotEnabled(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(false, nil).Once()
	// Remove should NOT be called

	err := alpine.DisableService(mockFS, testSvc)
	req.NoError(err)
}

func TestDisableService_RemoveError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", runLevelSvcLink).Return(true, nil).Once()
	mockFS.On("Remove", runLevelSvcLink).Return(errors.New("remove failed")).Once()

	err := alpine.DisableService(mockFS, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "failed to disable service")
}

// --- StartService ---

func TestStartService_NotStarted_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(false, nil).Once()
	mockFE.On("Run", false, "/sbin/rc-service", []string{testSvc, "start"}).
		Return([]byte("started"), nil).Once()

	err := alpine.StartService(mockFE, testSvc)
	req.NoError(err)
}

func TestStartService_AlreadyStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(true, nil).Once()
	// Run should NOT be called

	err := alpine.StartService(mockFE, testSvc)
	req.NoError(err)
}

func TestStartService_RunError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(false, nil).Once()
	mockFE.On("Run", false, "/sbin/rc-service", []string{testSvc, "start"}).
		Return(nil, errors.New("start failed")).Once()

	err := alpine.StartService(mockFE, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "error while starting service")
}

// --- StopService ---

func TestStopService_Started_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(true, nil).Once()
	mockFE.On("Run", false, "/sbin/rc-service", []string{testSvc, "stop"}).
		Return([]byte("stopped"), nil).Once()

	err := alpine.StopService(mockFE, testSvc)
	req.NoError(err)
}

func TestStopService_NotStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(false, nil).Once()
	// Run should NOT be called

	err := alpine.StopService(mockFE, testSvc)
	req.NoError(err)
}

func TestStopService_RunError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFE := host.NewMockFileExecutor(t)
	mockFE.On("Exists", startedSvcLink).Return(true, nil).Once()
	mockFE.On("Run", false, "/sbin/rc-service", []string{testSvc, "stop"}).
		Return(nil, errors.New("stop failed")).Once()

	err := alpine.StopService(mockFE, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "error while stopping service")
}

// --- PretendServiceStarted ---

func TestPretendServiceStarted_NotYet(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(false, nil).Once()
	mockFS.On("Symlink", initSvcFile, startedSvcLink).Return(nil).Once()

	err := alpine.PretendServiceStarted(mockFS, testSvc)
	req.NoError(err)
}

func TestPretendServiceStarted_AlreadyStarted(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(true, nil).Once()
	// Symlink should NOT be called

	err := alpine.PretendServiceStarted(mockFS, testSvc)
	req.NoError(err)
}

func TestPretendServiceStarted_SymlinkError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", startedSvcLink).Return(false, nil).Once()
	mockFS.On("Symlink", initSvcFile, startedSvcLink).Return(errors.New("symlink failed")).Once()

	err := alpine.PretendServiceStarted(mockFS, testSvc)
	req.Error(err)
	req.Contains(err.Error(), "failed to pretend service")
}

// --- EnsureOpenRCDirectory ---

func TestEnsureOpenRCDirectory_NotYet(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", openRCDir).Return(false, nil).Once()
	mockFS.On("Symlink", openRCSrcDir, openRCDir).Return(nil).Once()

	err := alpine.EnsureOpenRCDirectory(mockFS)
	req.NoError(err)
}

func TestEnsureOpenRCDirectory_AlreadyExists(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", openRCDir).Return(true, nil).Once()
	// Symlink should NOT be called

	err := alpine.EnsureOpenRCDirectory(mockFS)
	req.NoError(err)
}

func TestEnsureOpenRCDirectory_SymlinkError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockFS := host.NewMockFileSystem(t)
	mockFS.On("Exists", openRCDir).Return(false, nil).Once()
	mockFS.On("Symlink", openRCSrcDir, openRCDir).Return(errors.New("symlink failed")).Once()

	err := alpine.EnsureOpenRCDirectory(mockFS)
	req.Error(err)
	req.Contains(err.Error(), "failed to ensure OpenRC directory")
}
