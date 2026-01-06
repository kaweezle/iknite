package alpine

// cSpell: words RTNETLINK Nilf txeh
// cSpell: disable
import (
	"net"
	"os"
	"os/exec"
	"regexp"
	"testing"

	tu "github.com/kaweezle/iknite/pkg/testutils"
	"github.com/spf13/afero"
	"github.com/txn2/txeh"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/stretchr/testify/suite"
)

// cSpell: enable

type IPTestSuite struct {
	suite.Suite
	Executor    *tu.MockExecutor
	OldExecutor *utils.Executor
}

func (s *IPTestSuite) SetupTest() {
	s.Executor = &tu.MockExecutor{}
	s.OldExecutor = &utils.Exec
	utils.Exec = s.Executor
}

func (s *IPTestSuite) TeardownTest() {
	utils.Exec = *s.OldExecutor
}

func (s *IPTestSuite) TestIPExists() {
	require := s.Require()
	localhost := net.ParseIP("127.0.0.1")
	require.NotNil(localhost)

	result, err := CheckIpExists(localhost)
	require.NoError(err)
	require.True(result, "Localhost should exist")

	nonexistent := net.ParseIP("10.0.0.16")
	require.NotNil(nonexistent)

	result, err = CheckIpExists(nonexistent)
	require.NoError(err)
	require.False(result, "10.0.0.16 shouldn't exist")
}

func (s *IPTestSuite) TestAddIpAddress() {
	require := s.Require()

	ipaddr := net.ParseIP("192.168.99.2")
	require.NotNil(ipaddr)

	call := s.Executor.On("Run", true, "/sbin/ip", "addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0").
		Return("ok", nil)

	err := AddIpAddress("eth0", ipaddr)

	require.NoError(err)
	s.Executor.AssertExpectations(s.T())

	call.Unset()
	s.Executor.On("Run", true, "/sbin/ip", "addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0").
		Return("RTNETLINK answers: File exists", new(exec.ExitError))

	err = AddIpAddress("eth0", ipaddr)
	s.Executor.AssertExpectations(s.T())

	require.EqualError(err, "RTNETLINK answers: File exists: <nil>")
}

func (s *IPTestSuite) TestAddIpMapping() {
	require := s.Require()

	fs := afero.NewOsFs()
	afs := &afero.Afero{Fs: fs}

	f, err := os.CreateTemp("", "hosts")
	require.NoError(err)
	_ = f.Close()

	config := txeh.HostsConfig{
		ReadFilePath:  "testdata/hosts1",
		WriteFilePath: f.Name(),
	}

	ip := net.ParseIP("192.168.99.2")
	domainName := "kaweezle.local"

	err = AddIpMapping(&config, ip, domainName, []net.IP{})
	require.NoError(err)

	changed, err := afs.ReadFile(f.Name())
	require.NoError(err)

	re := regexp.MustCompile(`(?m)^192\.168\.99\.2\s+kaweezle.local$`)

	found := re.Find(changed)
	require.NotNilf(found, "File doesn't contain ip mapping:\n[%s]", string(changed))

	// Same test with the removal of an existing address
	config = txeh.HostsConfig{
		ReadFilePath:  "testdata/hosts2",
		WriteFilePath: f.Name(),
	}

	err = AddIpMapping(&config, ip, domainName, []net.IP{net.ParseIP("192.168.99.4")})
	require.NoError(err)
	require.NoError(err)

	changed, err = afs.ReadFile(f.Name())
	require.NoError(err)

	found = re.Find(changed)
	require.NotNil(found, "File doesn't contain ip mapping")

	re2 := regexp.MustCompile(`192\.168\.99\.4`)
	found = re2.Find(changed)
	require.Nil(found, "Shouldn't contain old IP")
}

func TestIP(t *testing.T) {
	suite.Run(t, new(IPTestSuite))
}
