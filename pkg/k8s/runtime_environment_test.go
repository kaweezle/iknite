package k8s_test

// cSpell: disable
import (
	"testing"

	"github.com/lithammer/dedent"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"

	"github.com/kaweezle/iknite/pkg/k8s"
	tu "github.com/kaweezle/iknite/pkg/testutils"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

type RuntimeEnvironmentTestSuite struct {
	suite.Suite
	Executor    *tu.MockExecutor
	OldExecutor *utils.Executor
}

func (s *RuntimeEnvironmentTestSuite) SetupTest() {
	s.Executor = &tu.MockExecutor{}
	s.OldExecutor = &utils.Exec
	utils.Exec = s.Executor
}

func (s *RuntimeEnvironmentTestSuite) TeardownTest() {
	utils.Exec = *s.OldExecutor
}

func (s *RuntimeEnvironmentTestSuite) TestPreventKubeletServiceFromStarting() {
	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	require := s.Require()
	fs := afero.NewOsFs()
	afs := &afero.Afero{Fs: fs}

	tempFile, err := afero.TempFile(fs, "", "rc.conf")
	require.NoError(err)
	defer func() {
		err = tempFile.Close()
	}()

	_, err = tempFile.WriteString(rcConfFileContent)
	require.NoError(err)

	confFilePath := tempFile.Name()

	err = k8s.PreventKubeletServiceFromStarting(confFilePath)
	require.NoError(err)

	content, err := afs.ReadFile(confFilePath)
	require.NoError(err)
	require.Equal(rcConfFileContent+k8s.RcConfPreventKubeletRunning+"\n", string(content))
}

func (s *RuntimeEnvironmentTestSuite) TestPreventKubeletServiceFromStartingWhenLineIsPresent() {
	// cSpell: disable
	existingContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    rc_kubelet_need="non-existing-service"
    `)
	// cSpell: enable

	require := s.Require()
	fs := afero.NewOsFs()
	afs := &afero.Afero{Fs: fs}

	tempFile, err := afero.TempFile(fs, "", "rc.conf")
	require.NoError(err)
	defer func() {
		err = tempFile.Close()
	}()

	_, err = tempFile.WriteString(existingContent)
	require.NoError(err)

	confFilePath := tempFile.Name()

	err = k8s.PreventKubeletServiceFromStarting(confFilePath)
	require.NoError(err)

	content, err := afs.ReadFile(confFilePath)
	require.NoError(err)
	require.Equal(existingContent, string(content))
}

func TestRuntimeEnvironment(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RuntimeEnvironmentTestSuite))
}
