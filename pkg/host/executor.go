package host

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/bitfield/script"
)

// ExecutorFunction is a function that executes a command and returns its input.
type ExecutorFunction = func(cmd string, arguments ...string) ([]byte, error)

// Executor provides methods to run commands and optionally pipe input to them.
type Executor interface {
	Run(combined bool, cmd string, arguments ...string) ([]byte, error)
	Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error)
	ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe
	ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe
}

// CommandExecutor executes commands on host environment.
type CommandExecutor struct{}

func (c *CommandExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
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

func (c *CommandExecutor) Pipe(
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

func (c *CommandExecutor) ExecPipe(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		stdin = script.NewPipe()
	}
	return stdin.Exec(cmd)
}

func (c *CommandExecutor) ExecForEach(stdin *script.Pipe, cmd string) *script.Pipe {
	if stdin == nil {
		return script.NewPipe().WithError(fmt.Errorf("stdin pipe cannot be nil"))
	}
	return stdin.ExecForEach(cmd)
}
