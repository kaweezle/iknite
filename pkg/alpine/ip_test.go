package alpine

import (
	"net"
	"os/exec"
	"testing"

	tu "github.com/kaweezle/iknite/pkg/testutils"

	"github.com/kaweezle/iknite/pkg/utils"
	"github.com/stretchr/testify/suite"
)

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

func TestIP(t *testing.T) {
	suite.Run(t, new(IPTestSuite))
}
