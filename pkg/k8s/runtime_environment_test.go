// cSpell: words testutils
package k8s_test

// cSpell: disable
import (
	"testing"

	"github.com/lithammer/dedent"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

// cSpell: words paralleltest

const confFilePath = "/etc/rc.conf"

func TestPreventKubeletServiceFromStarting(t *testing.T) {
	t.Parallel()
	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(rcConfFileContent), 0o644)
	req.NoError(err)

	err = k8s.PreventKubeletServiceFromStarting(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(rcConfFileContent+k8s.RcConfPreventKubeletRunning+"\n", string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestPreventKubeletServiceFromStarting_WhenLineIsPresent(t *testing.T) {
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
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(existingContent), 0o644)
	req.NoError(err)

	err = k8s.PreventKubeletServiceFromStarting(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(existingContent, string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestMakeIkniteServiceNeedNetworking(t *testing.T) {
	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(rcConfFileContent), 0o644)
	req.NoError(err)

	err = k8s.MakeIkniteServiceNeedNetworking(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(rcConfFileContent+k8s.RcConfIkniteNeedsNetworking+"\n", string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestMakeIkniteServiceNeedNetworking_WhenLineIsPresent(t *testing.T) {
	// cSpell: disable
	existingContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    rc_iknite_need="networking"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(existingContent), 0o644)
	req.NoError(err)

	err = k8s.MakeIkniteServiceNeedNetworking(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(existingContent, string(content))
}
