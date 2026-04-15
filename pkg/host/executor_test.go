// cSpell: words stretchr
package host_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestCommandExecutor_Run(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := &host.CommandExecutor{}

	out, err := exec.Run(false, "/bin/echo", "hello")
	req.NoError(err)
	req.Equal("hello\n", string(out))

	out, err = exec.Run(true, "/bin/sh", "-c", "echo -n err >&2; exit 7")
	req.Error(err)
	req.Contains(err.Error(), "failed to run command /bin/sh")
	req.Equal("err", strings.TrimSpace(string(out)))
}

func TestCommandExecutor_Pipe(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := &host.CommandExecutor{}
	out, err := exec.Pipe(strings.NewReader("abc"), false, "/bin/cat")
	req.NoError(err)
	req.Equal("abc", string(out))

	out, err = exec.Pipe(strings.NewReader("abc"), true, "/bin/sh", "-c", "cat >/dev/null; echo bad >&2; exit 3")
	req.Error(err)
	req.Contains(err.Error(), "failed to pipe command /bin/sh")
	req.Equal("bad", strings.TrimSpace(string(out)))
}
