// cSpell: words testutils cpuset netfilter modprobe paralleltest tparallel procs runlevels cyclop
package k8s_test

// cSpell: disable
import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/bitfield/script"
	"github.com/lithammer/dedent"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/txn2/txeh"

	mockHost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// cSpell: enable

const (
	cgroupSubtreeControlPath = "/sys/fs/cgroup/cgroup.subtree_control"
	cgroupProcsPath          = "/sys/fs/cgroup/cgroup.procs"
	cgroupInitDir            = "/sys/fs/cgroup/iknite_init"
	cgroupInitProcsPath      = "/sys/fs/cgroup/iknite_init/cgroup.procs"
	cgroupControllersPath    = "/sys/fs/cgroup/cgroup.controllers"
	machineIDPath            = "/etc/machine-id"
	netBridgePath            = "/proc/sys/net/bridge"
	ipForwardPath            = "/proc/sys/net/ipv4/ip_forward"
	bridgeNfCallPath         = "/proc/sys/net/bridge/bridge-nf-call-iptables"
	rpFilterPath             = "/proc/sys/net/ipv4/conf/default/rp_filter"
	runlevelIknitePath       = "/etc/runlevels/default/iknite"
)

const (
	confFilePath                = "/etc/rc.conf"
	RcConfPreventKubeletRunning = "rc_kubelet_need=\"non-existing-service\""
)

func TestPreventKubeletServiceFromStarting(t *testing.T) {
	t.Parallel()
	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(rcConfFileContent), 0o644)
	req.NoError(err)

	err = k8s.PreventServiceFromStarting(fs, confFilePath, "kubelet")
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(rcConfFileContent+RcConfPreventKubeletRunning+"\n", string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestPreventKubeletServiceFromStarting_WhenLineIsPresent(t *testing.T) {
	// cSpell: disable
	existingContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    rc_kubelet_need="non-existing-service"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(existingContent), 0o644)
	req.NoError(err)

	err = k8s.PreventServiceFromStarting(fs, confFilePath, "kubelet")
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(existingContent, string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestMakeIkniteServiceNeedNetworking(t *testing.T) {
	// cSpell: disable
	rcConfFileContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(rcConfFileContent), 0o644)
	req.NoError(err)

	err = k8s.MakeIkniteServiceNeedNetworking(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(rcConfFileContent+k8s.RcConfIkniteNeedsNetworking+"\n", string(content))
}

//nolint:paralleltest // Using a global variable util.Exec
func TestMakeIkniteServiceNeedNetworking_WhenLineIsPresent(t *testing.T) {
	// cSpell: disable
	existingContent := dedent.Dedent(`
    rc_sys="prefix"
    rc_controller_cgroups="NO"
    rc_depend_strict="NO"
    rc_need="!net !dev !udev-mount !sysfs !checkfs !fsck !netmount !logger !clock !modules"
    rc_iknite_need="networking"
    `)
	// cSpell: enable

	req := require.New(t)
	fs := host.NewMemMapFS()

	err := fs.WriteFile(confFilePath, []byte(existingContent), 0o644)
	req.NoError(err)

	err = k8s.MakeIkniteServiceNeedNetworking(fs, confFilePath)
	req.NoError(err)

	content, err := fs.ReadFile(confFilePath)
	req.NoError(err)
	req.Equal(existingContent, string(content))
}

func TestIsKubeletServiceRunnable(t *testing.T) {
	t.Parallel()
	// cSpell: disable
	rcConfWithKubelet := dedent.Dedent(`
    rc_sys="prefix"
    rc_kubelet_need="non-existing-service"
    `)
	rcConfWithoutKubelet := dedent.Dedent(`
    rc_sys="prefix"
    `)
	// cSpell: enable

	tests := []struct {
		setup        func(t *testing.T, fs host.FileSystem)
		name         string
		wantRunnable bool
		wantErr      bool
	}{
		{
			name: "kubelet line present means not runnable",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()
				require.NoError(t, fs.WriteFile(confFilePath, []byte(rcConfWithKubelet), 0o644))
			},
			wantRunnable: false,
		},
		{
			name: "kubelet line absent means runnable",
			setup: func(t *testing.T, fs host.FileSystem) {
				t.Helper()
				require.NoError(t, fs.WriteFile(confFilePath, []byte(rcConfWithoutKubelet), 0o644))
			},
			wantRunnable: true,
		},
		{
			name:    "missing file returns error",
			setup:   func(_ *testing.T, _ host.FileSystem) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := host.NewMemMapFS()
			tt.setup(t, fs)

			runnable, err := k8s.IsServiceRunnable(fs, confFilePath, "kubelet")
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.wantRunnable, runnable)
		})
	}
}

func TestEnsureConfigFileHasConfigurationLine_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		prepare         func(mfs *mockHost.MockFileSystem)
		wantErrContains string
	}{
		{
			name: "CountLines error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Pipe", confFilePath).
					Return(script.NewPipe().WithError(errors.New("read error"))).Once()
			},
			wantErrContains: "while checking",
		},
		{
			name: "Slice error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Pipe", confFilePath).
					Return(script.NewPipe().WithReader(strings.NewReader("existing\n"))).Once()
				mfs.On("Pipe", confFilePath).
					Return(script.NewPipe().WithError(errors.New("read error"))).Once()
			},
			wantErrContains: "while reading",
		},
		{
			name: "WriteFile error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Pipe", confFilePath).
					Return(script.NewPipe().WithReader(strings.NewReader("existing\n"))).Once()
				mfs.On("Pipe", confFilePath).
					Return(script.NewPipe().WithReader(strings.NewReader("existing\n"))).Once()
				mfs.On("WriteFile", confFilePath, mock.Anything, os.FileMode(0o644)).
					Return(errors.New("write failed")).Once()
			},
			wantErrContains: "while writing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			mfs := mockHost.NewMockFileSystem(t)
			tt.prepare(mfs)

			err := k8s.EnsureConfigFileHasConfigurationLine(mfs, confFilePath, "new-line")
			req.ErrorContains(err, tt.wantErrContains)
		})
	}
}

func TestEnsureNetworkInterfacesConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prepare func(mfs *mockHost.MockFileSystem)
		name    string
		wantErr bool
	}{
		{
			name: "file exists skips creation",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Exists", constants.NetworkInterfacesConfFile).Return(true, nil).Once()
			},
		},
		{
			name: "file absent creates it",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Exists", constants.NetworkInterfacesConfFile).Return(false, nil).Once()
				mfs.On("WriteFile", constants.NetworkInterfacesConfFile, mock.Anything, os.FileMode(0o644)).
					Return(nil).Once()
			},
		},
		{
			name: "Exists error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Exists", constants.NetworkInterfacesConfFile).
					Return(false, errors.New("stat error")).Once()
			},
			wantErr: true,
		},
		{
			name: "WriteFile error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("Exists", constants.NetworkInterfacesConfFile).Return(false, nil).Once()
				mfs.On("WriteFile", constants.NetworkInterfacesConfFile, mock.Anything, os.FileMode(0o644)).
					Return(errors.New("write error")).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			mfs := mockHost.NewMockFileSystem(t)
			tt.prepare(mfs)

			err := k8s.EnsureNetworkInterfacesConfiguration(mfs)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
		})
	}
}

func TestEnableCGroupSubtreeControl(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prepare func(mfs *mockHost.MockFileSystem)
		name    string
		wantErr bool
	}{
		{
			name: "already enabled is a no-op",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("+cpuset cpu"), nil).Once()
			},
		},
		{
			name: "enables with process migration and subtree write",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(nil).Once()
				mfs.On("Pipe", cgroupProcsPath).
					Return(script.NewPipe().WithReader(strings.NewReader("123\n"))).Once()
				mfs.On("WriteFile", cgroupInitProcsPath, []byte("123"), os.FileMode(0o644)).Return(nil).Once()
				mfs.On("ReadFile", cgroupControllersPath).Return([]byte("cpuset cpu"), nil).Once()
				mfs.On("WriteFile", cgroupSubtreeControlPath, mock.Anything, os.FileMode(0o644)).Return(nil).Once()
			},
		},
		{
			name: "process write warning does not abort",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(nil).Once()
				mfs.On("Pipe", cgroupProcsPath).
					Return(script.NewPipe().WithReader(strings.NewReader("456\n"))).Once()
				mfs.On("WriteFile", cgroupInitProcsPath, []byte("456"), os.FileMode(0o644)).
					Return(errors.New("move failed")).Once()
				mfs.On("ReadFile", cgroupControllersPath).Return([]byte("cpuset"), nil).Once()
				mfs.On("WriteFile", cgroupSubtreeControlPath, mock.Anything, os.FileMode(0o644)).Return(nil).Once()
			},
		},
		{
			name: "ReadFile subtree_control error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return(nil, errors.New("read error")).Once()
			},
			wantErr: true,
		},
		{
			name: "MkdirAll error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(errors.New("mkdir failed")).Once()
			},
			wantErr: true,
		},
		{
			name: "cgroup.procs Slice error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(nil).Once()
				mfs.On("Pipe", cgroupProcsPath).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			wantErr: true,
		},
		{
			name: "ReadFile cgroup.controllers error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(nil).Once()
				mfs.On("Pipe", cgroupProcsPath).
					Return(script.NewPipe().WithReader(strings.NewReader(""))).Once()
				mfs.On("ReadFile", cgroupControllersPath).Return(nil, errors.New("read error")).Once()
			},
			wantErr: true,
		},
		{
			name: "WriteFile subtree_control error propagates",
			prepare: func(mfs *mockHost.MockFileSystem) {
				mfs.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("cpu memory"), nil).Once()
				mfs.On("MkdirAll", cgroupInitDir, os.FileMode(0o755)).Return(nil).Once()
				mfs.On("Pipe", cgroupProcsPath).
					Return(script.NewPipe().WithReader(strings.NewReader(""))).Once()
				mfs.On("ReadFile", cgroupControllersPath).Return([]byte("cpuset"), nil).Once()
				mfs.On("WriteFile", cgroupSubtreeControlPath, mock.Anything, os.FileMode(0o644)).
					Return(errors.New("write error")).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			mfs := mockHost.NewMockFileSystem(t)
			tt.prepare(mfs)

			err := k8s.EnableCGroupSubtreeControl(mfs)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
		})
	}
}

// setupPreIPSuccessMocks mocks PrepareKubernetesEnvironment operations before IP configuration.
// All operations succeed.
func setupPreIPSuccessMocks(m *mockHost.MockHost) {
	m.On("WriteFile", ipForwardPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.On("Exists", netBridgePath).Return(true, nil).Once()
	m.On("WriteFile", bridgeNfCallPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.On("WriteFile", rpFilterPath, mock.Anything, mock.Anything).Return(nil).Once()
	m.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("+cpuset"), nil).Once()
	m.On("Exists", machineIDPath).Return(true, nil).Once()
}

// setupPostIPSuccessMocks mocks PrepareKubernetesEnvironment operations after IP configuration.
// All operations succeed.
func setupPostIPSuccessMocks(m *mockHost.MockHost) {
	m.On("Pipe", constants.RcConfFile).
		Return(script.NewPipe().WithReader(strings.NewReader(RcConfPreventKubeletRunning + "\n"))).Once()
	m.On("Exists", runlevelIknitePath).Return(true, nil).Once()
	m.On("Exists", constants.CrictlYaml).Return(true, nil).Once()
}

//nolint:cyclop // covers all PrepareKubernetesEnvironment branches
func TestPrepareKubernetesEnvironment(t *testing.T) {
	t.Parallel()
	ip := net.ParseIP("192.168.99.2")
	defaultConfig := &v1alpha1.IkniteClusterSpec{
		Ip:               ip,
		NetworkInterface: "eth0",
	}

	tests := []struct {
		name            string
		prepare         func(m *mockHost.MockHost)
		config          *v1alpha1.IkniteClusterSpec
		wantErrContains string
	}{
		{
			name: "happy path succeeds",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				setupPostIPSuccessMocks(m)
			},
		},
		{
			name: "logged WriteFile errors do not abort execution",
			prepare: func(m *mockHost.MockHost) {
				m.On("WriteFile", ipForwardPath, mock.Anything, mock.Anything).
					Return(errors.New("denied")).Once()
				m.On("Exists", netBridgePath).Return(true, nil).Once()
				m.On("WriteFile", bridgeNfCallPath, mock.Anything, mock.Anything).
					Return(errors.New("denied")).Once()
				m.On("WriteFile", rpFilterPath, mock.Anything, mock.Anything).
					Return(errors.New("denied")).Once()
				m.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("+cpuset"), nil).Once()
				m.On("Exists", machineIDPath).Return(true, nil).Once()
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				setupPostIPSuccessMocks(m)
			},
		},
		{
			name: "no outbound IP with networking succeeds then fails at kubelet",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(nil, errors.New("no route")).Once()
				// MakeIkniteServiceNeedNetworking: networking line not present yet.
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithReader(strings.NewReader("rc_sys=\"prefix\"\n"))).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithReader(strings.NewReader("rc_sys=\"prefix\"\n"))).Once()
				m.On("WriteFile", constants.RcConfFile, mock.Anything, os.FileMode(0o644)).Return(nil).Once()
				// PreventKubeletServiceFromStarting: fail here.
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			wantErrContains: "while preventing kubelet service from starting",
		},
		{
			name: "IP not bound, CreateIp true, AddIpAddress succeeds then fails at kubelet",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(false, nil).Once()
				m.On("Run", true, "/sbin/ip",
					[]string{"addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0"}).
					Return([]byte("ok"), nil).Once()
				// Fail at kubelet step.
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			config:          &v1alpha1.IkniteClusterSpec{Ip: ip, NetworkInterface: "eth0", CreateIp: true},
			wantErrContains: "while preventing kubelet service from starting",
		},
		{
			name: "domain name already mapped then fails at kubelet",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("IsHostMapped", mock.Anything, ip, "kaweezle.local").
					Return(true, []net.IP{ip}).Once()
				// Fail at kubelet step.
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			config:          &v1alpha1.IkniteClusterSpec{Ip: ip, DomainName: "kaweezle.local"},
			wantErrContains: "while preventing kubelet service from starting",
		},
		{
			name: "EnsureNetFilter error is returned",
			prepare: func(m *mockHost.MockHost) {
				m.On("WriteFile", ipForwardPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("Exists", netBridgePath).Return(false, nil).Once()
				m.On("Run", true, "/sbin/modprobe", []string{"br_netfilter"}).
					Return([]byte("error"), errors.New("modprobe failed")).Once()
			},
			wantErrContains: "while ensuring netfilter",
		},
		{
			name: "EnableCGroupSubtreeControl error is returned",
			prepare: func(m *mockHost.MockHost) {
				m.On("WriteFile", ipForwardPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("Exists", netBridgePath).Return(true, nil).Once()
				m.On("WriteFile", bridgeNfCallPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("WriteFile", rpFilterPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("ReadFile", cgroupSubtreeControlPath).Return(nil, errors.New("read error")).Once()
			},
			wantErrContains: "while enabling cgroup subtree control",
		},
		{
			name: "EnsureMachineID error is returned",
			prepare: func(m *mockHost.MockHost) {
				m.On("WriteFile", ipForwardPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("Exists", netBridgePath).Return(true, nil).Once()
				m.On("WriteFile", bridgeNfCallPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("WriteFile", rpFilterPath, mock.Anything, mock.Anything).Return(nil).Once()
				m.On("ReadFile", cgroupSubtreeControlPath).Return([]byte("+cpuset"), nil).Once()
				m.On("Exists", machineIDPath).Return(false, errors.New("stat error")).Once()
			},
			wantErrContains: "while ensuring machine ID",
		},
		{
			name: "no outbound IP and MakeIkniteServiceNeedNetworking fails",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(nil, errors.New("no route")).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			wantErrContains: "while ensuring IP configuration",
		},
		{
			name: "CheckIpExists error is returned",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(false, errors.New("check failed")).Once()
			},
			wantErrContains: "while ensuring IP configuration",
		},
		{
			name: "IP not bound and CreateIp false returns error",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(false, nil).Once()
			},
			config:          &v1alpha1.IkniteClusterSpec{Ip: ip, CreateIp: false},
			wantErrContains: "while ensuring IP configuration",
		},
		{
			name: "IP not bound, CreateIp true, AddIpAddress fails",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(false, nil).Once()
				m.On("Run", true, "/sbin/ip",
					[]string{"addr", "add", "192.168.99.2/24", "broadcast", "+", "dev", "eth0"}).
					Return([]byte("error"), errors.New("ip failed")).Once()
			},
			config:          &v1alpha1.IkniteClusterSpec{Ip: ip, NetworkInterface: "eth0", CreateIp: true},
			wantErrContains: "while ensuring IP configuration",
		},
		{
			name: "domain name not mapped and AddIpMapping fails",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("IsHostMapped", mock.Anything, ip, "kaweezle.local").
					Return(false, []net.IP{}).Once()
				m.On("GetHostsConfig").
					Return(&txeh.HostsConfig{ReadFilePath: "/nonexistent/hosts"}).Once()
			},
			config:          &v1alpha1.IkniteClusterSpec{Ip: ip, DomainName: "kaweezle.local"},
			wantErrContains: "while adding domain name",
		},
		{
			name: "PreventKubeletServiceFromStarting error is returned",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithError(errors.New("pipe error"))).Once()
			},
			wantErrContains: "while preventing kubelet service from starting",
		},
		{
			name: "EnableService error is returned",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithReader(
						strings.NewReader(RcConfPreventKubeletRunning + "\n"))).Once()
				m.On("Exists", runlevelIknitePath).Return(false, errors.New("stat error")).Once()
			},
			wantErrContains: "while enabling iknite service",
		},
		{
			name: "crictl.yaml existence error is returned",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithReader(
						strings.NewReader(RcConfPreventKubeletRunning + "\n"))).Once()
				m.On("Exists", runlevelIknitePath).Return(true, nil).Once()
				m.On("Exists", constants.CrictlYaml).Return(false, errors.New("stat error")).Once()
			},
			wantErrContains: "while ensuring",
		},
		{
			name: "crictl.yaml absent WriteFile fails returns error",
			prepare: func(m *mockHost.MockHost) {
				setupPreIPSuccessMocks(m)
				m.On("GetOutboundIP").Return(net.ParseIP("10.0.0.1"), nil).Once()
				m.On("CheckIpExists", mock.Anything).Return(true, nil).Once()
				m.On("Pipe", constants.RcConfFile).
					Return(script.NewPipe().WithReader(
						strings.NewReader(RcConfPreventKubeletRunning + "\n"))).Once()
				m.On("Exists", runlevelIknitePath).Return(true, nil).Once()
				m.On("Exists", constants.CrictlYaml).Return(false, nil).Once()
				m.On("WriteFile", constants.CrictlYaml, mock.Anything, mock.Anything).
					Return(errors.New("write failed")).Once()
			},
			wantErrContains: "while ensuring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			m := mockHost.NewMockHost(t)
			tt.prepare(m)

			cfg := defaultConfig
			if tt.config != nil {
				cfg = tt.config
			}

			err := k8s.PrepareKubernetesEnvironment(t.Context(), m, cfg)
			if tt.wantErrContains == "" {
				req.NoError(err)
				return
			}
			req.ErrorContains(err, tt.wantErrContains)
		})
	}
}
