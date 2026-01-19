package utils

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ExecutorFunction is a function that executes a command and returns its input.
type ExecutorFunction = func(cmd string, arguments ...string) ([]byte, error)

// The Executor interface provides a way to run commands and pipe input to them.
type Executor interface {
	Run(combined bool, cmd string, arguments ...string) ([]byte, error)
	Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error)
}

type CommandExecutor struct{}

func (c *CommandExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(context.Background(), cmd, arguments...)
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

var Exec Executor = &CommandExecutor{}
