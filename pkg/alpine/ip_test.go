package alpine_test

// cSpell: words RTNETLINK Nilf
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
	"github.com/kaweezle/iknite/pkg/host"
)

// cSpell: enable

func TestAddIpAddress(t *testing.T) {
	t.Parallel()

	req := require.New(t)

	ipaddr := net.ParseIP("192.168.99.2")
	req.NotNil(ipaddr)

	mockExec := host.NewMockExecutor(t)

	call := mockExec.On(
		"Run",
		true,
		"/sbin/ip",
		[]string{"addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0"},
	).Return([]byte("ok"), nil)

	err := alpine.AddIpAddress(mockExec, "eth0", ipaddr)
	req.NoError(err)
	mockExec.AssertExpectations(t)

	call.Unset()
	mockExec.On("Run", true, "/sbin/ip", []string{"addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0"}).
		Return([]byte("RTNETLINK answers: File exists"), new(exec.ExitError))

	err = alpine.AddIpAddress(mockExec, "eth0", ipaddr)
	req.EqualError(err, "RTNETLINK answers: File exists: <nil>")
	mockExec.AssertExpectations(t)
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
