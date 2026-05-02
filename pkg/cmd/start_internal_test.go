// cSpell: words ikniteapi procs runlevels cpuset testutil paralleltest
package cmd

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitfield/script"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	ikniteapi "github.com/kaweezle/iknite/pkg/apis/iknite"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
	"github.com/kaweezle/iknite/pkg/utils"
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

				persistClusterState(fs, ikniteapi.Initializing, []*v1alpha1.WorkloadState{{
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

				persistClusterState(fs, ikniteapi.Stabilizing, nil, nil)
			},
			wantReady: false,
		},
		{
			name: "stabilizing cluster with workloads is ready",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()

				persistClusterState(fs, ikniteapi.Stabilizing, []*v1alpha1.WorkloadState{{
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

const (
	cgroupSubtreeControlPath    = "/sys/fs/cgroup/cgroup.subtree_control"
	machineIDPath               = "/etc/machine-id"
	netBridgePath               = "/proc/sys/net/bridge"
	ipForwardPath               = "/proc/sys/net/ipv4/ip_forward"
	bridgeNfCallPath            = "/proc/sys/net/bridge/bridge-nf-call-iptables"
	rpFilterPath                = "/proc/sys/net/ipv4/conf/default/rp_filter"
	runlevelIknitePath          = "/etc/runlevels/default/iknite"
	RcConfPreventKubeletRunning = "rc_kubelet_need=\"non-existing-service\""
)

func persistClusterState(
	fs host.FileSystem,
	state ikniteapi.ClusterState,
	ready []*v1alpha1.WorkloadState,
	unready []*v1alpha1.WorkloadState,
) {
	cluster := v1alpha1.NewDefaultIkniteCluster()
	cluster.Update(state, "phase-a", ready, unready, fs)
}

func setupPrepareSuccessMocks(m *mockHost.MockHost) {
	m.EXPECT().WriteFile(ipForwardPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.EXPECT().Exists(netBridgePath).Return(true, nil).Once()
	m.EXPECT().WriteFile(bridgeNfCallPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.EXPECT().WriteFile(rpFilterPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.EXPECT().ReadFile(cgroupSubtreeControlPath).Return([]byte("+cpuset"), nil).Once()
	m.EXPECT().Exists(machineIDPath).Return(true, nil).Once()
	m.EXPECT().GetOutboundIP().Return(net.ParseIP("10.0.0.1"), nil).Once()
	m.EXPECT().CheckIpExists(mock.Anything).Return(true, nil).Once()
	m.EXPECT().Pipe(constants.RcConfFile).
		Return(script.NewPipe().WithReader(strings.NewReader(RcConfPreventKubeletRunning + "\n"))).Once()
	m.EXPECT().Exists(runlevelIknitePath).Return(true, nil).Once()
	m.EXPECT().Exists(constants.CrictlYaml).Return(true, nil).Once()
}

func setupFirstStartMocks(t *testing.T, m *mockHost.MockHost, configExists bool) {
	t.Helper()
	if configExists {
		content, err := testutil.GetBasicConfigContent("https://192.168.99.2:6443")
		require.NoError(t, err)
		m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).Return(content, nil).Once()
	} else {
		m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).Return(nil, os.ErrNotExist).Once()
	}
	m.EXPECT().Run(true, "/sbin/openrc", []string{"default"}).Return([]byte("ok"), nil).Once()
	count := 0
	statuses := []struct {
		ready   []*v1alpha1.WorkloadState
		unready []*v1alpha1.WorkloadState
		state   ikniteapi.ClusterState
	}{
		{
			state: ikniteapi.Initializing,
			ready: []*v1alpha1.WorkloadState{{
				Namespace: "kube-system",
				Name:      "flannel",
				Ok:        true,
				Message:   "ready",
			}},
			unready: nil,
		},
		{
			state: ikniteapi.Stabilizing,
			ready: []*v1alpha1.WorkloadState{{
				Namespace: "kube-system",
				Name:      "flannel",
				Ok:        true,
				Message:   "ready",
			}},
			unready: nil,
		},
	}
	fs := host.NewMemMapFS()
	m.EXPECT().ReadFile(constants.StatusFile).RunAndReturn(func(path string) ([]byte, error) {
		if count < 0 {
			count++
			return nil, os.ErrNotExist
		}
		if count >= len(statuses) {
			return nil, errors.New("no more statuses")
		}
		state := statuses[count]
		persistClusterState(fs, state.state, state.ready, state.unready)
		count++
		return fs.ReadFile(path)
	})
}

func TestPerformStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*testing.T, *mockHost.MockHost, *v1alpha1.IkniteClusterSpec, *utils.WaitOptions)
		wantErr string
	}{
		{
			name: "successful cold start with no existing configuration",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				setupFirstStartMocks(t, m, false)
			},
		},
		{
			name: "successful cold start with existing configuration",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				setupFirstStartMocks(t, m, true)
			},
		},
		{
			name:    "prepare fails",
			wantErr: "failed to prepare kubernetes environment",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				m.EXPECT().WriteFile(ipForwardPath, mock.Anything, mock.Anything).
					Return(nil).Once()
				m.EXPECT().Exists(netBridgePath).Return(false, errors.New("failed to check net bridge")).Once()
			},
		},
		{
			name:    "Change server address on existing cluster",
			wantErr: "kubeconfig server address does not match iknite config API endpoint",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				content, err := testutil.GetBasicConfigContent("https://different-server:6443")
				require.NoError(t, err)
				m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).
					Return(content, nil).Once()
			},
		},
		{
			name:    "Error on configuration load failure",
			wantErr: "failed to load existing cluster admin.conf",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).
					Return(nil, errors.New("failed to load existing cluster admin.conf")).Once()
			},
		},
		{
			name:    "Fail to start OpenRC",
			wantErr: "failed to start OpenRC",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, _ *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).
					Return(nil, os.ErrNotExist).Once()
				m.EXPECT().Run(true, "/sbin/openrc", []string{"default"}).
					Return(nil, errors.New("failed to start OpenRC")).Once()
			},
		},
		{
			name:    "Fail to Poll",
			wantErr: "cluster did not become ready in time",
			setup: func(t *testing.T, m *mockHost.MockHost, _ *v1alpha1.IkniteClusterSpec, w *utils.WaitOptions) {
				t.Helper()
				setupPrepareSuccessMocks(m)
				m.EXPECT().ReadFile(kubeadmConstants.GetAdminKubeConfigPath()).
					Return(nil, os.ErrNotExist).Once()
				m.EXPECT().Run(true, "/sbin/openrc", []string{"default"}).
					Return([]byte("ok"), nil).Once()
				w.Watch = true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := require.New(t)

			ip := net.ParseIP("192.168.99.2")
			defaultConfig := &v1alpha1.IkniteClusterSpec{
				Ip:               ip,
				NetworkInterface: "eth0",
			}
			m := mockHost.NewMockHost(t)
			waitOptions := &utils.WaitOptions{
				Timeout:      0,
				CheckTimeout: 1 * time.Second,
				Interval:     100 * time.Millisecond,
				Retries:      0,
				OkResponses:  1,
				Watch:        false,
				Wait:         true,
				Immediate:    true,
			}
			if tt.setup != nil {
				tt.setup(t, m, defaultConfig, waitOptions)
			}

			err := performStart(t.Context(), m, defaultConfig, waitOptions)
			if tt.wantErr != "" {
				req.Error(err)
				req.ErrorContains(err, tt.wantErr)
				return
			}
			req.NoError(err)
		})
	}
}

//nolint:paralleltest // current viper usage prevents parallelization
func TestStartCommand(t *testing.T) {
	req := require.New(t)

	ip := net.ParseIP("192.168.99.2")
	defaultConfig := &v1alpha1.IkniteClusterSpec{
		Ip:               ip,
		NetworkInterface: "eth0",
	}
	m := mockHost.NewMockHost(t)
	waitOptions := &utils.WaitOptions{
		Timeout:      0,
		CheckTimeout: 1 * time.Second,
		Interval:     100 * time.Millisecond,
		Retries:      0,
		OkResponses:  1,
		Watch:        false,
		Wait:         true,
		Immediate:    true,
	}
	setupPrepareSuccessMocks(m)
	setupFirstStartMocks(t, m, false)

	command := NewStartCmd(defaultConfig, waitOptions, m)
	err := command.ExecuteContext(t.Context())
	req.NoError(err)
}
