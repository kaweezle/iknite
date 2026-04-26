package k8s

// cSpell: words ipcns utsns testmount tmpfs mountpoint

import (
	"errors"
	"os"
	"testing"

	"github.com/bitfield/script"
	"github.com/stretchr/testify/require"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
)

const testMountPath = "/testmount"

// --- processMounts ---

func TestProcessMounts_EvalSymlinksNotExist(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return("", os.ErrNotExist).Once()

	err := processMounts(mockH, testMountPath, false, "test", false)
	req.NoError(err)
}

func TestProcessMounts_EvalSymlinksOtherError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return("", errors.New("permission denied")).Once()

	err := processMounts(mockH, testMountPath, false, "test", false)
	req.Error(err)
	req.Contains(err.Error(), "failed to evaluate symlinks")
}

func TestProcessMounts_NoMatchingMounts(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	// Pipe returns mounts with no matching path
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs /other tmpfs rw 0 0\n")).Once()

	err := processMounts(mockH, testMountPath, false, "test", false)
	req.NoError(err)
}

func TestProcessMounts_DryRun_WithMatch(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	// Pipe returns a mount matching our path (column 2 is the mountpoint)
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs " + testMountPath + " tmpfs rw 0 0\n")).Once()
	// No Unmount call expected (dry run)

	err := processMounts(mockH, testMountPath, false, "Unmounting", true /* isDryRun */)
	req.NoError(err)
}

func TestProcessMounts_Unmount_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs " + testMountPath + " tmpfs rw 0 0\n")).Once()
	mockH.On("Unmount", testMountPath).Return(nil).Once()

	err := processMounts(mockH, testMountPath, false /* remove */, "Unmounting", false)
	req.NoError(err)
}

func TestProcessMounts_Unmount_Error_ContinuesAndReturnsNil(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs " + testMountPath + " tmpfs rw 0 0\n")).Once()
	// Unmount fails, but processMounts logs and continues
	mockH.On("Unmount", testMountPath).Return(errors.New("device busy")).Once()

	// unmount error is logged but not propagated (p.Error() from script pipe is nil)
	err := processMounts(mockH, testMountPath, false, "Unmounting", false)
	req.NoError(err)
}

func TestProcessMounts_Remove_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs " + testMountPath + " tmpfs rw 0 0\n")).Once()
	mockH.On("Unmount", testMountPath).Return(nil).Once()
	mockH.On("RemoveAll", testMountPath).Return(nil).Once()

	err := processMounts(mockH, testMountPath, true /* remove */, "Unmounting and removing", false)
	req.NoError(err)
}

func TestProcessMounts_Remove_Error_ContinuesAndReturnsNil(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.Echo("tmpfs " + testMountPath + " tmpfs rw 0 0\n")).Once()
	mockH.On("Unmount", testMountPath).Return(nil).Once()
	// RemoveAll fails, logged but not propagated
	mockH.On("RemoveAll", testMountPath).Return(errors.New("remove failed")).Once()

	err := processMounts(mockH, testMountPath, true, "Unmounting and removing", false)
	req.NoError(err)
}

func TestProcessMounts_PipeError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockH := mockHost.NewMockHost(t)
	mockH.On("EvalSymlinks", testMountPath).Return(testMountPath, nil).Once()
	// Pipe itself returns an error pipe (e.g., /proc/self/mounts unreadable)
	mockH.On("Pipe", "/proc/self/mounts").
		Return(script.NewPipe().WithError(errors.New("read error"))).Once()

	err := processMounts(mockH, testMountPath, false, "Unmounting", false)
	req.Error(err)
	req.Contains(err.Error(), "failed to process mounts")
}
