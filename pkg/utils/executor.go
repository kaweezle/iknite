package utils

import (
	"io"
	"os/exec"
)

// ExecutorFunction is a function that executes a command and returns its input
type ExecutorFunction = func(cmd string, arguments ...string) ([]byte, error)

// The Executor interface provides a way to run commands and pipe input to them
type Executor interface {
	Run(combined bool, cmd string, arguments ...string) ([]byte, error)
	Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error)
}

type CommandExecutor struct {
}

func (c *CommandExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
	command := exec.Command(cmd, arguments...)
	if combined {
		return command.CombinedOutput()
	} else {
		return command.Output()
	}
}

func (c *CommandExecutor) Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error) {
	command := exec.Command(cmd, arguments...)
	command.Stdin = stdin
	if combined {
		return command.CombinedOutput()
	} else {
		return command.Output()
	}
}

var Exec Executor = &CommandExecutor{}
