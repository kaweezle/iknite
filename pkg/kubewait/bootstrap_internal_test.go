// cSpell: words chmoded
package kubewait

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunBootstrapVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepare func(t *testing.T) *Options
		name    string
		wantErr bool
	}{
		{
			name: "missing script is skipped",
			prepare: func(t *testing.T) *Options {
				t.Helper()
				dir := t.TempDir()
				return &Options{BootstrapOptions: BootstrapOptions{BootstrapDir: dir, BootstrapScript: "missing.sh"}}
			},
			wantErr: false,
		},
		{
			name: "non executable script is chmoded and run",
			prepare: func(t *testing.T) *Options {
				t.Helper()
				req := require.New(t)
				dir := t.TempDir()
				script := filepath.Join(dir, "iknite-bootstrap.sh")
				req.NoError(os.WriteFile(script, []byte("#!/bin/sh\necho done > run.ok\n"), 0o600))
				return &Options{
					BootstrapOptions: BootstrapOptions{BootstrapDir: dir, BootstrapScript: filepath.Base(script)},
				}
			},
			wantErr: false,
		},
		{
			name: "script error is returned",
			prepare: func(t *testing.T) *Options {
				t.Helper()
				req := require.New(t)
				dir := t.TempDir()
				script := filepath.Join(dir, "iknite-bootstrap.sh")
				req.NoError(os.WriteFile(script, []byte("#!/bin/sh\nexit 9\n"), 0o600))
				req.NoError(os.Chmod(script, 0o755)) //nolint:gosec // test script needs executable bit
				return &Options{
					BootstrapOptions: BootstrapOptions{BootstrapDir: dir, BootstrapScript: filepath.Base(script)},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			opts := tt.prepare(t)
			err := runBootstrap(context.Background(), opts)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)

			if tt.name == "non executable script is chmoded and run" {
				_, statErr := os.Stat(filepath.Join(opts.BootstrapDir, "run.ok"))
				req.NoError(statErr)
			}
		})
	}
}

func TestEnsureSSHKnownHostBranches(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	err := ensureSSHKnownHost(context.Background(), "https://example.com/repo.git")
	req.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = ensureSSHKnownHost(ctx, "git@github.com:owner/repo.git")
	req.Error(err)
	req.Contains(err.Error(), "did not become resolvable")
}
