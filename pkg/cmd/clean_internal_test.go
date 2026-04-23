// cSpell: words ipcns utsns
//
//nolint:errcheck,forcetypeassert // Ignoring error checks in tests for simplicity
package cmd

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
)

var simpleDummyHostOptions = host.DummyHostOptions{
	Processes: []host.DummyProcessOptions{
		{Pid: 1, Cmd: "init", State: host.ProcessStateRunning, ExitCode: 0},
		{Pid: 2, Cmd: "containerd", State: host.ProcessStateRunning, ExitCode: 0},
		{Pid: 3, Cmd: "iknite", State: host.ProcessStateRunning, ExitCode: 0},
		{Pid: 4, Cmd: "kubelet", State: host.ProcessStateRunning, ExitCode: 0},
	},
	// cSpell: disable
	//nolint:lll // raw output
	FakeOutputs: map[string]string{
		`ip -j link show`: `[{"ifindex":1,"ifname":"lo","flags":["LOOPBACK","UP","LOWER_UP"],"mtu":65536,"qdisc":"noqueue","operstate":"UNKNOWN","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"loopback","address":"00:00:00:00:00:00","broadcast":"00:00:00:00:00:00"},{"ifindex":2,"ifname":"cni0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","operstate":"UP","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"ether","address":"f2:ae:13:c4:19:b7","broadcast":"ff:ff:ff:ff:ff:ff"},{"ifindex":3,"link_index":2,"ifname":"vethceed7ade","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"1e:1f:4f:db:d8:9f","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":1},{"ifindex":4,"link_index":2,"ifname":"vethb9f80ab9","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"46:c2:2c:ef:f3:1e","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":2},{"ifindex":5,"link_index":2,"ifname":"veth68980f08","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"86:aa:27:3a:eb:11","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":3},{"ifindex":6,"link_index":2,"ifname":"veth3341ea56","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"0e:f9:2a:a9:ef:17","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":4},{"ifindex":7,"link_index":8,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"noqueue","operstate":"UP","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"ether","address":"10:66:6a:b7:73:6a","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":0},{"ifindex":8,"link_index":2,"ifname":"veth5c1f73c8","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"72:86:8d:29:01:c0","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":5},{"ifindex":9,"link_index":2,"ifname":"vetha5a5a5d6","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"aa:06:f3:f4:87:54","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":6},{"ifindex":10,"link_index":2,"ifname":"veth10451890","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"12:2b:ed:54:be:bc","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":7},{"ifindex":11,"link_index":2,"ifname":"veth5340dd0f","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"0e:8d:cd:11:c1:15","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":8},{"ifindex":12,"link_index":2,"ifname":"veth47858b2c","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"f2:e7:fd:26:f9:9f","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":9},{"ifindex":13,"link_index":2,"ifname":"veth3df28555","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"5a:f0:9d:f7:47:41","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":10},{"ifindex":14,"link_index":2,"ifname":"vethb5e52524","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"1a:1b:6d:46:15:93","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":11},{"ifindex":15,"link_index":2,"ifname":"veth776cc963","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"da:b0:ab:ba:40:4b","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":12},{"ifindex":16,"link_index":2,"ifname":"vethd652ff33","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","master":"cni0","operstate":"UP","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"da:0c:e0:b6:32:ea","broadcast":"ff:ff:ff:ff:ff:ff","link_netnsid":13},{"ifindex":17,"ifname":"flannel.1","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1450,"qdisc":"noqueue","operstate":"UNKNOWN","linkmode":"DEFAULT","group":"default","link_type":"ether","address":"4e:3e:d7:11:85:84","broadcast":"ff:ff:ff:ff:ff:ff"}]`,
		`ip -br -4 a sh`: `lo               UNKNOWN        127.0.0.1/8
cni0             UP             10.244.0.1/24
eth0@if8         UP             10.253.141.236/24 192.168.99.2/24
flannel.1        UNKNOWN        10.244.0.0/32
`,
	},
	// cSpell: enable
	Mounts: []string{
		"/var/lib/kubelet/pods",
		"/var/lib/kubelet/plugins",
		"/var/lib/kubelet",
		"/run/containerd",
		"/run/netns",
		"/run/ipcns",
		"/run/utsns",
	},
	NetworkIPs:   []net.IP{net.ParseIP("10.253.141.51"), net.ParseIP("192.168.99.2")},
	HostMappings: map[string][]string{"192.168.99.2": {"iknite.local"}},
}

var simpleIkniteConfig = &v1alpha1.IkniteClusterSpec{
	Ip:         net.ParseIP("192.168.99.2"),
	CreateIp:   true,
	DomainName: "iknite.local",
}

const removeIpAddressCmd = "ip addr del 192.168.99.2/24 dev eth0@if8"

func createCleanParameters(t *testing.T) (host.Host, *v1alpha1.IkniteClusterSpec, *cleanOptions, error) {
	t.Helper()
	fs := host.NewMemMapFS()
	err := fs.MkdirAll("/run/iknite", 0o755)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create directory in mem fs: %w", err)
	}
	content, err := os.ReadFile("testdata/iknite_status.json")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read testdata file: %w", err)
	}
	err = fs.WriteFile("/run/iknite/status.json", content, os.FileMode(0o644))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to write testdata file to mem fs: %w", err)
	}
	alpineHost, err := host.NewDummyHost(fs, &simpleDummyHostOptions)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create dummy host: %w", err)
	}
	ikniteConfig := *simpleIkniteConfig
	v1alpha1.SetDefaults_IkniteClusterSpec(&ikniteConfig)

	cleanOptions := newCleanOptions()
	return alpineHost, &ikniteConfig, cleanOptions, nil
}

func TestPerformClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepareParameters func(t *testing.T) (host.Host, *v1alpha1.IkniteClusterSpec, *cleanOptions, error)
		expectations      func(req *require.Assertions, alpineHost host.Host)
		name              string
		expectedError     string
	}{
		{
			name: "nominal dry-run clean",
			prepareParameters: func(t *testing.T) (
				host.Host,
				*v1alpha1.IkniteClusterSpec,
				*cleanOptions,
				error,
			) {
				t.Helper()
				alpineHost, ikniteConfig, cleanOptions, err := createCleanParameters(t)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("failed to prepare parameters: %w", err)
				}
				cleanOptions.dryRun = true
				return alpineHost, ikniteConfig, cleanOptions, nil
			},
			expectedError: "",
			expectations: func(req *require.Assertions, alpineHost host.Host) {
				dummyHost := alpineHost.(*host.DelegateHost)
				dummyExec := dummyHost.Exec.(*host.DummyExecutor)
				// Check that no processes were killed
				for _, process := range dummyExec.Processes {
					dummyProcess := process.(*host.DummyProcess)
					req.Nil(
						dummyProcess.State(),
						"Process %s should not have been killed in dry-run mode",
						dummyProcess.String(),
					)
				}
				// Check that all mounts and directories are still present
				for _, mount := range simpleDummyHostOptions.Mounts {
					exists, err := alpineHost.Exists(mount)
					req.NoError(err, "Error while checking existence of mount %s", mount)
					req.True(exists, "Mount %s should still be present in dry-run mode", mount)
				}
				dummySystem := dummyHost.Sys.(*host.DummySystem)
				for _, mount := range simpleDummyHostOptions.Mounts {
					req.Contains(dummySystem.Mounts, mount, "Mount %s should still be present in dry-run mode", mount)
				}
			},
		},
		{
			name:              "nominal non dry-run clean",
			prepareParameters: createCleanParameters,
			expectedError:     "",
			expectations: func(req *require.Assertions, alpineHost host.Host) {
				dummyHost := alpineHost.(*host.DelegateHost)
				dummyExec := dummyHost.Exec.(*host.DummyExecutor)
				// Check that no processes were killed except kubelet
				for _, process := range dummyExec.Processes {
					dummyProcess := process.(*host.DummyProcess)
					switch dummyProcess.Cmd() {
					case "kubelet":
						req.NotNil(dummyProcess.State(), "Process kubelet should have been killed in non dry-run mode")
						req.Contains(dummyProcess.String(),
							"state=Terminated", "Process kubelet should be in exited state")
						req.Equal(0, dummyProcess.State().ExitCode(), "Process kubelet should have exit code 0")
					default:
						req.Nil(
							dummyProcess.State(),
							"Process %s should not have been killed in non dry-run mode",
							dummyProcess.Cmd(),
						)
					}
				}
				dummySys := dummyHost.Sys.(*host.DummySystem)
				req.Empty(
					dummySys.Mounts,
					"All mounts should have been removed in non dry-run mode (%d mounts still present)",
					len(dummySys.Mounts),
				)

				// Check that the IP address has not been removed
				req.NotContains(
					dummyExec.GetCalledCommands(),
					removeIpAddressCmd,
					"IP address should not have been removed in non dry-run mode",
				)
				// Check that iknite has been stopped
				req.Contains(
					dummyExec.GetCalledCommands(),
					"/sbin/rc-service iknite stop",
					"iknite service should have been stopped in non dry-run mode",
				)
			},
		},
		{
			name: "clean all",
			prepareParameters: func(t *testing.T) (
				host.Host,
				*v1alpha1.IkniteClusterSpec,
				*cleanOptions,
				error,
			) {
				t.Helper()
				alpineHost, ikniteConfig, cleanOptions, err := createCleanParameters(t)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("failed to prepare parameters: %w", err)
				}
				cleanOptions.cleanAll = true
				return alpineHost, ikniteConfig, cleanOptions, nil
			},
			expectations: func(req *require.Assertions, alpineHost host.Host) {
				dummyHost := alpineHost.(*host.DelegateHost)
				dummyExec := dummyHost.Exec.(*host.DummyExecutor)
				// Check that the IP address has been removed
				req.Contains(
					dummyExec.GetCalledCommands(),
					removeIpAddressCmd,
					"IP address should have been removed in clean all mode",
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			alpineHost, ikniteConfig, cleanOptions, err := tt.prepareParameters(t)
			req.NoError(err)

			cmd := NewCmdClean(ikniteConfig, cleanOptions, alpineHost)
			err = cmd.ExecuteContext(t.Context())
			if tt.expectedError != "" {
				req.ErrorContains(err, tt.expectedError)
			} else {
				req.NoError(err)
			}

			if tt.expectations != nil {
				tt.expectations(req, alpineHost)
			}
		})
	}
}
