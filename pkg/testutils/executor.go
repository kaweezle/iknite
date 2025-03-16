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

func (m *MockExecutor) Run(combined bool, cmd string, arguments ...string) ([]byte, error) {

	items := append(make([]interface{}, 0), combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}

func (m *MockExecutor) Pipe(stdin io.Reader, combined bool, cmd string, arguments ...string) ([]byte, error) {
	items := append(make([]interface{}, 0), stdin, combined, cmd)
	for _, arg := range arguments {
		items = append(items, arg)
	}
	args := m.Called(items...)
	return []byte(args.String(0)), args.Error(1)
}
