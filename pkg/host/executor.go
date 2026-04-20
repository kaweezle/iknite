// cSpell: words wrapcheck
package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/bitfield/script"
)

type CommandOptions struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Cmd    string
	Dir    string
	Args   []string
	Env    []string
}

type Process interface {
	Pid() int
	Signal(signal os.Signal) error
	Wait() error
	State() *os.ProcessState
}

// ExecutorFunction is a function that executes a command and returns its input.
type ExecutorFunction = func(cmd string, arguments ...string) ([]byte, error)

// Executor provides methods to run commands and optionally pipe input to them.
type Executor interface {
	Run(combined bool, cmd string, arguments ...string) ([]byte, error)
	PipeRun(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error)
	ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe
	ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe
	FindProcess(pid int) (Process, error)
	StartCommand(ctx context.Context, options *CommandOptions) (Process, error)
}

var _ Executor = (*hostImpl)(nil)

// findProcessFn wraps os.FindProcess so it can be replaced in tests to
// simulate errors (os.FindProcess never fails on Linux).
var findProcessFn = os.FindProcess

func NewDefaultExecutor() Executor {
	return NewOsFS().(*hostImpl) //nolint:errcheck,forcetypeassert // Good type
}

func (c *hostImpl) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(
		context.Background(),
		cmd,
		arguments...)
	var output []byte
	var err error
	if combined {
		output, err = command.CombinedOutput()
	} else {
		output, err = command.Output()
	}
	if err != nil {
		return output, fmt.Errorf("failed to run command %s: %w", cmd, err)
	}
	return output, nil
}

func (c *hostImpl) PipeRun(
	stdin io.Reader,
	combined bool,
	cmd string,
	arguments ...string,
) ([]byte, error) {
	command := exec.CommandContext(context.Background(), cmd, arguments...)
	command.Stdin = stdin
	var output []byte
	var err error
	if combined {
		output, err = command.CombinedOutput()
	} else {
		output, err = command.Output()
	}
	if err != nil {
		return output, fmt.Errorf("failed to pipe command %s: %w", cmd, err)
	}
	return output, nil
}

func (c *hostImpl) ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		stdin = script.NewPipe()
	}
	return stdin.Exec(cmd)
}

func (c *hostImpl) ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		return script.NewPipe().WithError(fmt.Errorf("stdin pipe cannot be nil"))
	}
	return stdin.ExecForEach(cmd)
}

type processImpl struct {
	process *os.Process
	state   *os.ProcessState
}

func (p *processImpl) Pid() int {
	return p.process.Pid
}

func (p *processImpl) Signal(signal os.Signal) error {
	return p.process.Signal(signal) //nolint:wrapcheck // Want to return original error
}

func (p *processImpl) Wait() error {
	var err error
	p.state, err = p.process.Wait()
	return err //nolint:wrapcheck // Want to return original error
}

func (p *processImpl) State() *os.ProcessState {
	return p.state
}

func (c *hostImpl) FindProcess(pid int) (Process, error) {
	process, err := findProcessFn(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to find process with pid %d: %w", pid, err)
	}
	return &processImpl{process: process}, nil
}

//nolint:gosec // Harness done upstream
func (c *hostImpl) StartCommand(ctx context.Context, options *CommandOptions) (Process, error) {
	cmd := exec.CommandContext(ctx, options.Cmd, options.Args...)
	cmd.Env = options.Env
	cmd.Dir = options.Dir
	cmd.Stdout = options.Stdout
	cmd.Stderr = options.Stderr
	cmd.Stdin = options.Stdin
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", options.Cmd, err)
	}
	return &processImpl{process: cmd.Process}, nil
}

func TerminateProcess(p Process, alive *bool) error {
	if p == nil {
		return fmt.Errorf("process cannot be nil")
	}
	err := p.Signal(syscall.SIGTERM)

	if alive != nil {
		*alive = false
	}

	if err != nil {
		return fmt.Errorf("failed to terminate process: %w", err)
	}
	err = p.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for process termination: %w", err)
	}

	return nil
}
