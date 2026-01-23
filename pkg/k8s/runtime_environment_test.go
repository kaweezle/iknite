package k8s_test

// cSpell: disable
import (
	"testing"

	"github.com/lithammer/dedent"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/k8s"
	tu "github.com/kaweezle/iknite/pkg/testutils"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func setupExecutor(t *testing.T) func() {
	t.Helper()
	executor := &tu.MockExecutor{}
	old := utils.Exec
	utils.Exec = executor
	oldFS := utils.FS
	utils.FS = utils.NewMemMapFS()
	return func() {
		utils.Exec = old
		utils.FS = oldFS
	}
}

const confFilePath = "/etc/rc.conf"

func TestPreventKubeletServiceFromStarting(t *testing.T) {
	teardown := setupExecutor(t)
	defer teardown()

	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	req := require.New(t)

	err := utils.FS.WriteFile(confFilePath, []byte(rcConfFileContent), 0o644)
	req.NoError(err)

	err = k8s.PreventKubeletServiceFromStarting(confFilePath)
	req.NoError(err)

	content, err := utils.FS.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(rcConfFileContent+k8s.RcConfPreventKubeletRunning+"\n", string(content))
}

func TestPreventKubeletServiceFromStarting_WhenLineIsPresent(t *testing.T) {
	teardown := setupExecutor(t)
	defer teardown()

	// cSpell: disable
	existingContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    rc_kubelet_need="non-existing-service"
    `)
	// cSpell: enable

	req := require.New(t)

	err := utils.FS.WriteFile(confFilePath, []byte(existingContent), 0o644)
	req.NoError(err)

	err = k8s.PreventKubeletServiceFromStarting(confFilePath)
	req.NoError(err)

	content, err := utils.FS.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(existingContent, string(content))
}
