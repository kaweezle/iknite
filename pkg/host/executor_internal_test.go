// cSpell: words paralleltest
package host

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates package-level findProcessFn; cannot run in parallel
func TestFindProcess_Error(t *testing.T) {
	req := require.New(t)

	original := findProcessFn
	defer func() { findProcessFn = original }()

	findProcessFn = func(_ int) (*os.Process, error) {
		return nil, errors.New("process not found")
	}

	exec := NewDefaultExecutor()
	_, err := exec.FindProcess(99999)
	req.Error(err)
	req.Contains(err.Error(), "failed to find process with pid")
}
