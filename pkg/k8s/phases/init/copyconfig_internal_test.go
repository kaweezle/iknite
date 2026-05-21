package init

import (
	"testing"

	"github.com/stretchr/testify/require"

	mockInit "github.com/kaweezle/iknite/mocks/pkg/k8s/phases/init"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

func TestRunCopyConfig_BadData(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	err := runCopyConfig("bad data")
	req.Error(err)
	req.Contains(err.Error(), "copy-config phase invoked with an invalid data struct")
}

func TestRunCopyConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepare    func(req *require.Assertions, fs host.FileSystem)
		assertions func(req *require.Assertions, fs host.FileSystem)
		name       string
		wantErr    string
	}{
		{
			name: "successfully copies admin.conf and iknite.conf",
			prepare: func(req *require.Assertions, fs host.FileSystem) {
				req.NoError(testutil.CreateBasicConfig(fs, "", ""))
				req.NoError(testutil.CreateTestCA(fs, ""))
			},
			assertions: func(req *require.Assertions, fs host.FileSystem) {
				ok, err := fs.Exists(constants.KubernetesRootConfig)
				req.NoError(err)
				req.True(ok, "expected config file to exist at %s", constants.KubernetesRootConfig)
				ok, err = fs.Exists(constants.IkniteLocalConfPath)
				req.NoError(err)
				req.True(ok, "expected iknite config file to exist at %s", constants.IkniteLocalConfPath)
			},
		},
		{
			name:    "fails if admin.conf doesn't exist",
			wantErr: "failed to copy admin.conf",
		},
		{
			name: "fails if no ca cert exists",
			prepare: func(req *require.Assertions, fs host.FileSystem) {
				req.NoError(testutil.CreateBasicConfig(fs, "", ""))
			},
			wantErr: "failed to ensure iknite server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			m := mockInit.NewMockCopyConfigData(t)
			fs := host.NewMemMapFS()
			alpineHost, ok := fs.(host.Host)
			req.True(ok, "expected MemMapFS to implement Host")
			m.EXPECT().Logger().Return(testutil.TestLogger(t)).Once()
			m.EXPECT().IkniteCluster().Return(&v1alpha1.IkniteCluster{
				Spec: v1alpha1.IkniteClusterSpec{
					ClusterName: "test-cluster",
				},
			}).Once()
			m.EXPECT().Host().Return(alpineHost).Once()

			if tt.prepare != nil {
				tt.prepare(req, fs)
			}

			err := runCopyConfig(m)
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err, "expected no error from runCopyConfig")
				if tt.assertions != nil {
					tt.assertions(req, fs)
				}
			}
		})
	}
}
