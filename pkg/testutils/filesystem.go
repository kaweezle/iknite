// cSpell: words testutils
package testutils

import "github.com/kaweezle/iknite/pkg/utils"

func CreateTestFS() (utils.FileSystem, func()) {
	oldFS := utils.FS
	cleanup := func() {
		utils.FS = oldFS
	}
	utils.FS = utils.NewMemMapFS()
	return utils.FS, cleanup
}

func CreateTestFSAndExecutor() (utils.FileSystem, *MockExecutor, func()) {
	oldFS := utils.FS
	utils.FS = utils.NewMemMapFS()
	mockExec := &MockExecutor{}
	oldExec := utils.Exec
	utils.Exec = mockExec
	cleanup := func() {
		utils.FS = oldFS
		utils.Exec = oldExec
	}
	return utils.FS, mockExec, cleanup
}
