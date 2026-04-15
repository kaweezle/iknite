// cSpell: words testutils stretchr
package testutils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/testutils"
)

func TestMockExecutorRunAndPipe(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := &testutils.MockExecutor{}
	mockExec.On("Run", false, "echo", "a", "b").Return("ok", nil).Once()
	out, err := mockExec.Run(false, "echo", "a", "b")
	req.NoError(err)
	req.Equal("ok", string(out))

	stdin := strings.NewReader("payload")
	mockExec.On("Pipe", stdin, true, "cat", "-n").Return("done", nil).Once()
	out, err = mockExec.Pipe(stdin, true, "cat", "-n")
	req.NoError(err)
	req.Equal("done", string(out))

	mockExec.AssertExpectations(t)
}
