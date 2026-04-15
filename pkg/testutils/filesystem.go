// cSpell: words testutils
package testutils

import (
	"testing"

	"github.com/kaweezle/iknite/pkg/host"
)

func CreateTestFS() (host.FileSystem, func()) {
	oldFS := host.FS
	cleanup := func() {
		host.FS = oldFS
	}
	host.FS = host.NewMemMapFS()
	return host.FS, cleanup
}

func CreateTestFSAndExecutor(t *testing.T) (host.FileSystem, *host.MockExecutor, func()) {
	t.Helper()
	oldFS := host.FS
	host.FS = host.NewMemMapFS()
	mockExec := host.NewMockExecutor(t)
	oldExec := host.Exec
	host.Exec = mockExec
	cleanup := func() {
		host.FS = oldFS
		host.Exec = oldExec
	}
	return host.FS, mockExec, cleanup
}
