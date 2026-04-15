// cspell: words stretchr testutils netfilter paralleltest
package alpine

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	tu "github.com/kaweezle/iknite/pkg/testutils"
)

//nolint:paralleltest // tests modify global state
func TestEnsureNetFilter(t *testing.T) {
	tests := []struct {
		prepare func(t *testing.T, mockExec *tu.MockExecutor)
		name    string
		wantErr bool
	}{
		{
			name: "directory exists skips modprobe",
			prepare: func(t *testing.T, _ *tu.MockExecutor) {
				t.Helper()
				req := require.New(t)
				req.NoError(host.FS.MkdirAll(brNetfilterDir, 0o755))
			},
			wantErr: false,
		},
		{
			name: "missing directory runs modprobe",
			prepare: func(_ *testing.T, mockExec *tu.MockExecutor) {
				mockExec.On("Run", true, "/sbin/modprobe", netfilter_module).Return("ok", nil).Once()
			},
			wantErr: false,
		},
		{
			name: "modprobe error is returned",
			prepare: func(_ *testing.T, mockExec *tu.MockExecutor) {
				mockExec.On("Run", true, "/sbin/modprobe", netfilter_module).Return("boom", errors.New("failed")).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			_, mockExec, cleanup := tu.CreateTestFSAndExecutor()
			t.Cleanup(cleanup)

			tt.prepare(t, mockExec)

			err := EnsureNetFilter()
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			mockExec.AssertExpectations(t)
		})
	}
}

//nolint:paralleltest // tests modify global state
func TestEnsureMachineID(t *testing.T) {
	req := require.New(t)

	_, cleanup := tu.CreateTestFS()
	t.Cleanup(cleanup)

	req.NoError(host.FS.MkdirAll("/etc", 0o755))

	req.NoError(EnsureMachineID())

	exists, err := host.FS.Exists(machineIDFile)
	req.NoError(err)
	req.True(exists)

	before, err := host.FS.ReadFile(machineIDFile)
	req.NoError(err)
	req.NotEmpty(before)

	req.NoError(EnsureMachineID())
	after, err := host.FS.ReadFile(machineIDFile)
	req.NoError(err)
	req.Equal(string(before), string(after))
}
