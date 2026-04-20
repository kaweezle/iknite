package host_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/bitfield/script"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestCommandExecutor_Run(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

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

	exec := host.NewDefaultExecutor()
	out, err := exec.PipeRun(strings.NewReader("abc"), false, "/bin/cat")
	req.NoError(err)
	req.Equal("abc", string(out))

	out, err = exec.PipeRun(strings.NewReader("abc"), true, "/bin/sh", "-c", "cat >/dev/null; echo bad >&2; exit 3")
	req.Error(err)
	req.Contains(err.Error(), "failed to pipe command /bin/sh")
	req.Equal("bad", strings.TrimSpace(string(out)))
}

func TestCommandExecutor_ExecPipe(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

	// nil stdin creates a new pipe internally
	p := exec.ExecPipe(nil, "/bin/echo hello")
	req.NotNil(p)
	str, err := p.String()
	req.NoError(err)
	req.Equal("hello\n", str)

	// non-nil stdin is chained
	pipe := script.Echo("world")
	p2 := exec.ExecPipe(pipe, "/bin/cat")
	req.NotNil(p2)
	str, err = p2.String()
	req.NoError(err)
	req.Equal("world", str)
}

func TestCommandExecutor_ExecForEach(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

	// nil stdin returns an error pipe
	p := exec.ExecForEach(nil, "/bin/echo {{.}}")
	req.NotNil(p)
	req.Error(p.Error())

	// non-nil stdin executes the template for each line
	pipe := script.Slice([]string{"alpha", "beta"})
	p2 := exec.ExecForEach(pipe, "/bin/echo {{.}}")
	req.NotNil(p2)
	str, err := p2.String()
	req.NoError(err)
	req.Contains(str, "alpha")
	req.Contains(str, "beta")
}

func TestCommandExecutor_FindProcess(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

	// Find the current process by PID
	proc, err := exec.FindProcess(os.Getpid())
	req.NoError(err)
	req.NotNil(proc)
	req.Equal(os.Getpid(), proc.Pid())

	// Signal 0 checks process existence without delivering a real signal
	err = proc.Signal(syscall.Signal(0))
	req.NoError(err)
}

func TestCommandExecutor_StartCommand(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

	var stdout strings.Builder
	proc, err := exec.StartCommand(context.Background(), &host.CommandOptions{
		Cmd:    "/bin/echo",
		Args:   []string{"hello"},
		Stdout: &stdout,
	})
	req.NoError(err)
	req.NotNil(proc)
	req.Positive(proc.Pid())

	// State is nil before Wait
	req.Nil(proc.State())

	err = proc.Wait()
	req.NoError(err)

	// State is populated after Wait
	req.NotNil(proc.State())
	req.True(proc.State().Success())
}

func TestCommandExecutor_StartCommand_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	exec := host.NewDefaultExecutor()

	_, err := exec.StartCommand(context.Background(), &host.CommandOptions{
		Cmd: "/nonexistent/command",
	})
	req.Error(err)
	req.Contains(err.Error(), "failed to start command")
}

func TestTerminateProcess(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	p := host.NewMockProcess(t)
	p.On("Signal", syscall.SIGTERM).Return(nil).Once()
	p.On("Wait").Return(nil).Once()

	alive := true
	err := host.TerminateProcess(p, &alive)
	req.NoError(err)
	req.False(alive)
}

func TestTerminateProcess_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	p := host.NewMockProcess(t)
	p.On("Signal", syscall.SIGTERM).Return(fmt.Errorf("signal error")).Once()

	alive := true
	err := host.TerminateProcess(p, &alive)
	req.Error(err)
	req.Contains(err.Error(), "failed to terminate process")
	req.False(alive) // alive should remain true if termination failed
}

func TestTerminateProcess_WaitError(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	p := host.NewMockProcess(t)
	p.On("Signal", syscall.SIGTERM).Return(nil).Once()
	p.On("Wait").Return(fmt.Errorf("wait error")).Once()

	alive := true
	err := host.TerminateProcess(p, &alive)
	req.Error(err)
	req.Contains(err.Error(), "failed to wait for process termination")
	req.False(alive) // alive should remain true if wait failed
}
