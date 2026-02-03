package alpine_test

// cSpell: words RTNETLINK Nilf txeh
// cSpell: disable
import (
	"net"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/txn2/txeh"

	"github.com/kaweezle/iknite/pkg/alpine"
	tu "github.com/kaweezle/iknite/pkg/testutils"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

func setupExecutor(t *testing.T) (*tu.MockExecutor, func()) {
	t.Helper()
	executor := &tu.MockExecutor{}
	old := utils.Exec
	utils.Exec = executor
	return executor, func() {
		utils.Exec = old
	}
}

func TestIPExists(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	localhost := net.ParseIP("127.0.0.1")
	req.NotNil(localhost)

	result, err := alpine.CheckIpExists(localhost)
	req.NoError(err)
	req.True(result, "Localhost should exist")

	nonexistent := net.ParseIP("10.0.0.16")
	req.NotNil(nonexistent)

	result, err = alpine.CheckIpExists(nonexistent)
	req.NoError(err)
	req.False(result, "10.0.0.16 shouldn't exist")
}

func TestAddIpAddress(t *testing.T) {
	t.Parallel()
	executor, teardown := setupExecutor(t)
	defer teardown()

	req := require.New(t)

	ipaddr := net.ParseIP("192.168.99.2")
	req.NotNil(ipaddr)

	call := executor.On("Run", true, "/sbin/ip", "addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0").
		Return("ok", nil)

	err := alpine.AddIpAddress("eth0", ipaddr)

	req.NoError(err)
	executor.AssertExpectations(t)

	call.Unset()
	executor.On("Run", true, "/sbin/ip", "addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0").
		Return("RTNETLINK answers: File exists", new(exec.ExitError))

	err = alpine.AddIpAddress("eth0", ipaddr)
	executor.AssertExpectations(t)

	req.EqualError(err, "RTNETLINK answers: File exists: <nil>")
}

func TestAddIpMapping(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := afero.NewOsFs()
	afs := &afero.Afero{Fs: fs}

	f, err := os.CreateTemp("", "hosts")
	req.NoError(err)
	req.NoError(f.Close())

	config := txeh.HostsConfig{
		ReadFilePath:  "testdata/hosts1",
		WriteFilePath: f.Name(),
	}

	ip := net.ParseIP("192.168.99.2")
	domainName := "kaweezle.local"

	err = alpine.AddIpMapping(&config, ip, domainName, []net.IP{})
	req.NoError(err)

	changed, err := afs.ReadFile(f.Name())
	req.NoError(err)

	re := regexp.MustCompile(`(?m)^192\.168\.99\.2\s+kaweezle.local$`)

	found := re.Find(changed)
	req.NotNilf(found, "File doesn't contain ip mapping:\n[%s]", string(changed))

	// Same test with the removal of an existing address
	config = txeh.HostsConfig{
		ReadFilePath:  "testdata/hosts2",
		WriteFilePath: f.Name(),
	}

	err = alpine.AddIpMapping(&config, ip, domainName, []net.IP{net.ParseIP("192.168.99.4")})
	req.NoError(err)
	req.NoError(err)

	changed, err = afs.ReadFile(f.Name())
	req.NoError(err)

	found = re.Find(changed)
	req.NotNil(found, "File doesn't contain ip mapping")

	re2 := regexp.MustCompile(`192\.168\.99\.4`)
	found = re2.Find(changed)
	req.Nil(found, "Shouldn't contain old IP")
}
