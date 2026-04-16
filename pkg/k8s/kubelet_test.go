// cSpell: words cgroupfs
package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	//nolint:lll // Ignore long line linter warning
	kubeletEnvFileContent = `
command_args="--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf --cgroup-driver=cgroupfs --config=/var/lib/kubelet/config.yaml"
`
	kubeAdmFlagsFileContent = `
KUBELET_KUBEADM_ARGS="--node-ip=192.168.99.2"
`
)

func TestReadEnvFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		prepare func(req *require.Assertions, fs host.FileSystem) ([]string, error)
		assert  func(req *require.Assertions, got map[string]string)
		wantErr string
	}{
		{
			name: "nominal use case",
			prepare: func(req *require.Assertions, fs host.FileSystem) ([]string, error) {
				err := fs.WriteFile("/etc/conf.d/kubelet", []byte(kubeletEnvFileContent), 0o644)
				req.NoError(err, "failed to write kubelet env file")
				err = fs.WriteFile("/etc/conf.d/kubeadm-flags", []byte(kubeAdmFlagsFileContent), 0o644)
				req.NoError(err, "failed to write kubeadm flags file")
				return []string{"/etc/conf.d/kubelet", "/etc/conf.d/kubeadm-flags"}, nil
			},
			assert: func(req *require.Assertions, got map[string]string) {
				//nolint:lll // Ignore long line linter warning
				req.Equal(
					"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf --cgroup-driver=cgroupfs --config=/var/lib/kubelet/config.yaml",
					got["command_args"],
					"unexpected value for command_args",
				)
				req.Equal(
					"--node-ip=192.168.99.2",
					got["KUBELET_KUBEADM_ARGS"],
					"unexpected value for KUBELET_KUBEADM_ARGS",
				)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := host.NewMemMapFS()
			paths, err := tt.prepare(req, fs)
			req.NoError(err, "error while preparing paths")

			got, gotErr := k8s.ReadEnvFiles(fs, paths...)
			if gotErr != nil {
				if tt.wantErr == "" {
					req.Error(gotErr)
					req.ErrorContains(gotErr, tt.wantErr)
					t.Errorf("ReadEnvFiles() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatal("ReadEnvFiles() succeeded unexpectedly")
			}
			if tt.assert != nil {
				tt.assert(req, got)
			}
		})
	}
}
