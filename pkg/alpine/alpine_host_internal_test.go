// cspell: words stretchr testutils netfilter paralleltest
package alpine

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
)

func TestEnsureNetFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prepare func(t *testing.T, fs *host.MockFileSystem, mockExec *host.MockExecutor)
		name    string
		wantErr bool
	}{
		{
			name: "directory exists skips modprobe",
			prepare: func(_ *testing.T, fs *host.MockFileSystem, _ *host.MockExecutor) {
				fs.On("Exists", brNetfilterDir).Return(true, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "missing directory runs modprobe",
			prepare: func(_ *testing.T, fs *host.MockFileSystem, mockExec *host.MockExecutor) {
				fs.On("Exists", brNetfilterDir).Return(false, nil).Once()
				mockExec.On("Run", true, "/sbin/modprobe", []string{netfilter_module}).Return([]byte("ok"), nil).Once()
			},
			wantErr: false,
		},
		{
			name: "modprobe error is returned",
			prepare: func(_ *testing.T, fs *host.MockFileSystem, mockExec *host.MockExecutor) {
				fs.On("Exists", brNetfilterDir).Return(false, nil).Once()
				mockExec.On("Run", true, "/sbin/modprobe", []string{netfilter_module}).
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

			mockExec := host.NewMockExecutor(t)
			mockFs := host.NewMockFileSystem(t)
			h := NewAlpineHost(mockFs, mockExec, host.NewMockNetworkHost(t), host.NewMockSystem(t))

			tt.prepare(t, mockFs, mockExec)

			err := h.EnsureNetFilter()
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
	h := NewAlpineHost(fs, host.NewMockExecutor(t), host.NewMockNetworkHost(t), host.NewMockSystem(t))

	req.NoError(fs.MkdirAll("/etc", 0o755))

	req.NoError(h.EnsureMachineID())

	exists, err := fs.Exists(machineIDFile)
	req.NoError(err)
	req.True(exists)

	before, err := fs.ReadFile(machineIDFile)
	req.NoError(err)
	req.NotEmpty(before)

	req.NoError(h.EnsureMachineID())
	after, err := fs.ReadFile(machineIDFile)
	req.NoError(err)
	req.Equal(string(before), string(after))
}
