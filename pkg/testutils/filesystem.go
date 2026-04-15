// cSpell: words testutils
package testutils

import "github.com/kaweezle/iknite/pkg/host"

func CreateTestFS() (host.FileSystem, func()) {
	oldFS := host.FS
	cleanup := func() {
		host.FS = oldFS
	}
	host.FS = host.NewMemMapFS()
	return host.FS, cleanup
}

func CreateTestFSAndExecutor() (host.FileSystem, *MockExecutor, func()) {
	oldFS := host.FS
	host.FS = host.NewMemMapFS()
	mockExec := &MockExecutor{}
	oldExec := host.Exec
	host.Exec = mockExec
	cleanup := func() {
		host.FS = oldFS
		host.Exec = oldExec
	}
	return host.FS, mockExec, cleanup
}
