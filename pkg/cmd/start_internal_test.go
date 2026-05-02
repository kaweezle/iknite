package cmd

// cSpell: words ikniteapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	ikniteapi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
)

func TestIsIkniteReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup       func(*testing.T, host.FileSystem)
		name        string
		wantErrText string
		wantReady   bool
	}{
		{
			name:      "missing status file returns not ready",
			wantReady: false,
		},
		{
			name: "invalid status file returns wrapped error",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()

				req := require.New(t)
				req.NoError(fs.MkdirAll(constants.StatusDirectory, 0o755))
				req.NoError(fs.WriteFile(constants.StatusFile, []byte("not-json"), 0o644))
			},
			wantErrText: "failed to load iknite cluster",
			wantReady:   false,
		},
		{
			name: "initializing cluster with workloads is not ready",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()

				persistClusterState(t, fs, ikniteapi.Initializing, []*v1alpha1.WorkloadState{{
					Namespace: "kube-system",
					Name:      "flannel",
					Ok:        true,
					Message:   "ready",
				}}, nil)
			},
			wantReady: false,
		},
		{
			name: "stabilizing cluster without workloads is not ready",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()

				persistClusterState(t, fs, ikniteapi.Stabilizing, nil, nil)
			},
			wantReady: false,
		},
		{
			name: "stabilizing cluster with workloads is ready",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()

				persistClusterState(t, fs, ikniteapi.Stabilizing, []*v1alpha1.WorkloadState{{
					Namespace: "kube-system",
					Name:      "metrics-server",
					Ok:        true,
					Message:   "ready",
				}}, []*v1alpha1.WorkloadState{{
					Namespace: "kube-system",
					Name:      "local-path-provisioner",
					Ok:        false,
					Message:   "waiting",
				}})
			},
			wantReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := require.New(t)
			fs := host.NewMemMapFS()
			if tt.setup != nil {
				tt.setup(t, fs)
			}

			ready, err := IsIkniteReady(context.Background(), fs)

			if tt.wantErrText != "" {
				req.Error(err)
				req.ErrorContains(err, tt.wantErrText)
			} else {
				req.NoError(err)
			}
			req.Equal(tt.wantReady, ready)
		})
	}
}

func persistClusterState(
	t *testing.T,
	fs host.FileSystem,
	state ikniteapi.ClusterState,
	ready []*v1alpha1.WorkloadState,
	unready []*v1alpha1.WorkloadState,
) {
	t.Helper()

	cluster := v1alpha1.NewDefaultIkniteCluster()
	cluster.Update(state, "phase-a", ready, unready, fs)
}
