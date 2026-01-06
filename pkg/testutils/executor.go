// cSpell: words testutils
package testutils

// cSpell: disable
import (
	"io"

	"github.com/stretchr/testify/mock"
)

// cSpell: enable

type MockExecutor struct {
	mock.Mock
}

// Run executes a command with the specified arguments and returns the output or an error.
//
// Parameters:
//   - combined: A boolean indicating whether to combine stdout and stderr.
//   - cmd: The command to be executed as a string.
//   - arguments: A variadic slice of strings representing the command arguments.
//
// Returns:
//   - A byte slice containing the command output.
//   - An error if the command execution fails.
func (m *MockExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {
	items := append(make([]any, 0), combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}

// Pipe executes a command with the provided arguments and optionally combines
// the standard output and standard error streams. It allows for input to be
// passed via an io.Reader.
//
// Parameters:
//   - stdin: An io.Reader providing input to the command.
//   - combined: A boolean indicating whether to combine stdout and stderr.
//   - cmd: The command to execute.
//   - arguments: Additional arguments to pass to the command.
//
// Returns:
//   - A byte slice containing the command's output.
//   - An error if the command execution fails.
func (m *MockExecutor) Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error) {
	items := append(make([]any, 0), stdin, combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}
