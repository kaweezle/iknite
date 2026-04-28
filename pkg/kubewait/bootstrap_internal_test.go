// cSpell: words chmoded testutil
package kubewait

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const bootstrapDir = "/base"

func TestRunBootstrapVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepare      func(t *testing.T, fs host.FileSystem, exec *mockHost.MockExecutor) *Options
		expectations func(req *require.Assertions, opts *Options, fs host.FileSystem)
		name         string
		wantErr      bool
	}{
		{
			name: "missing script is skipped",
			prepare: func(t *testing.T, _ host.FileSystem, _ *mockHost.MockExecutor) *Options {
				t.Helper()
				return &Options{
					BootstrapOptions: BootstrapOptions{BootstrapDir: bootstrapDir, BootstrapScript: "missing.sh"},
				}
			},
			wantErr: false,
		},
		{
			name: "non executable script is chmoded and run",
			prepare: func(t *testing.T, fs host.FileSystem, exec *mockHost.MockExecutor) *Options {
				t.Helper()
				req := require.New(t)
				script := filepath.Join(bootstrapDir, "iknite-bootstrap.sh")
				req.NoError(fs.WriteFile(script, []byte("#!/bin/sh\necho done > run.ok\n"), 0o600))
				exec.EXPECT().
					RunCommand(t.Context(), mock.Anything).
					RunAndReturn(func(_ context.Context, _ *host.CommandOptions) error {
						err := fs.WriteFile(filepath.Join(bootstrapDir, "run.ok"), []byte("done"), 0o600)
						if err != nil {
							return fmt.Errorf("failed to write run.ok: %w", err)
						}
						return nil
					}).
					Once()
				return &Options{
					BootstrapOptions: BootstrapOptions{
						BootstrapDir:    bootstrapDir,
						BootstrapScript: filepath.Base(script),
					},
				}
			},
			wantErr: false,
			expectations: func(req *require.Assertions, opts *Options, fs host.FileSystem) {
				_, statErr := fs.Stat(filepath.Join(opts.BootstrapDir, "run.ok"))
				req.NoError(statErr)
			},
		},
		{
			name: "script error is returned",
			prepare: func(t *testing.T, fs host.FileSystem, exec *mockHost.MockExecutor) *Options {
				t.Helper()
				req := require.New(t)
				script := filepath.Join(bootstrapDir, "iknite-bootstrap.sh")
				req.NoError(fs.WriteFile(script, []byte("#!/bin/sh\nexit 9\n"), 0o600))
				req.NoError(fs.Chmod(script, 0o755))
				exec.EXPECT().RunCommand(t.Context(), mock.Anything).Return(testutil.FakeExec("", 9)).Once()
				return &Options{
					BootstrapOptions: BootstrapOptions{
						BootstrapDir:    bootstrapDir,
						BootstrapScript: filepath.Base(script),
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			fs := host.NewMemMapFS()
			mockExecutor := mockHost.NewMockExecutor(t)
			opts := tt.prepare(t, fs, mockExecutor)
			h := &testutil.DelegateHost{Fs: fs, Exec: mockExecutor}
			err := runBootstrap(t.Context(), h, opts)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			if tt.expectations != nil {
				tt.expectations(req, opts, fs)
			}
		})
	}
}

func TestEnsureSSHKnownHostBranches(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	h, err := testutil.NewDummyHost(host.NewMemMapFS(), &testutil.DummyHostOptions{})
	req.NoError(err)
	err = ensureSSHKnownHost(t.Context(), h, "https://example.com/repo.git")
	req.NoError(err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = ensureSSHKnownHost(ctx, h, "git@github.com:owner/repo.git")
	req.Error(err)
	req.Contains(err.Error(), "did not become resolvable")
}
