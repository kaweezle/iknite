// cspell: words stretchr testutils netfilter paralleltest
package alpine

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/host"
)

func TestEnsureNetFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prepare func(t *testing.T, mockExec *mockHost.MockFileExecutor)
		name    string
		wantErr bool
	}{
		{
			name: "directory exists skips modprobe",
			prepare: func(_ *testing.T, fs *mockHost.MockFileExecutor) {
				fs.On("Exists", brNetfilterDir).Return(true, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "missing directory runs modprobe",
			prepare: func(_ *testing.T, mockExec *mockHost.MockFileExecutor) {
				mockExec.On("Exists", brNetfilterDir).Return(false, nil).Once()
				mockExec.On("Run", true, modProbeCmd, []string{netfilter_module}).Return([]byte("ok"), nil).Once()
			},
			wantErr: false,
		},
		{
			name: "modprobe error is returned",
			prepare: func(_ *testing.T, mockExec *mockHost.MockFileExecutor) {
				mockExec.On("Exists", brNetfilterDir).Return(false, nil).Once()
				mockExec.On("Run", true, modProbeCmd, []string{netfilter_module}).
					Return([]byte("boom"), errors.New("failed")).
					Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			mockFileExec := mockHost.NewMockFileExecutor(t)

			tt.prepare(t, mockFileExec)

			err := EnsureNetFilter(mockFileExec)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
		})
	}
}

//nolint:paralleltest // tests modify global state
func TestEnsureMachineID(t *testing.T) {
	req := require.New(t)

	fs := host.NewMemMapFS()

	req.NoError(fs.MkdirAll("/etc", 0o755))

	req.NoError(EnsureMachineID(fs))

	exists, err := fs.Exists(machineIDFile)
	req.NoError(err)
	req.True(exists)

	before, err := fs.ReadFile(machineIDFile)
	req.NoError(err)
	req.NotEmpty(before)

	req.NoError(EnsureMachineID(fs))
	after, err := fs.ReadFile(machineIDFile)
	req.NoError(err)
	req.Equal(string(before), string(after))
}
