package k8s_test

// cSpell: words txeh ifname wrapcheck

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitfield/script"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/txn2/txeh"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

// --- ResetIPAddress ---

func TestResetIPAddress_CreateIpFalse(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}

	// No methods should be called on the mock
	err := k8s.ResetIPAddress(mockHost, config, false)
	req.NoError(err)
}

func TestResetIPAddress_HostNotMapped(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	tmpDir := t.TempDir()
	hostsFile, writeErr := os.CreateTemp(tmpDir, "hosts")
	req.NoError(writeErr)
	defer hostsFile.Close() //nolint:errcheck // ignore close error in test
	_, writeErr = hostsFile.WriteString("127.0.0.1 localhost\n")
	req.NoError(writeErr)

	hostsConfig := &txeh.HostsConfig{
		ReadFilePath:  hostsFile.Name(),
		WriteFilePath: hostsFile.Name(),
	}

	mockHost := host.NewMockHost(t)
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "nonexistent.local",
	}

	// IpMappingForHost fails → logs warning and returns nil
	err := k8s.ResetIPAddress(mockHost, config, false)
	req.NoError(err)
}

func newHostsFileConfig(t *testing.T, content string) *txeh.HostsConfig {
	t.Helper()
	tmpDir := t.TempDir()
	hostsFile, err := os.CreateTemp(tmpDir, "hosts")
	require.NoError(t, err)
	hostsFile.Close() //nolint:errcheck // best-effort close in test
	require.NoError(t, os.WriteFile(hostsFile.Name(), []byte(content), 0o600))
	return &txeh.HostsConfig{ReadFilePath: hostsFile.Name(), WriteFilePath: hostsFile.Name()}
}

func TestResetIPAddress_IPFoundDryRunCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		execForEachCmd string
		isDryRun       bool
	}{
		{
			name:           "dry run",
			isDryRun:       true,
			execForEachCmd: "echo ip addr del 192.168.99.2/24 dev {{.}}",
		},
		{
			name:           "not dry run",
			isDryRun:       false,
			execForEachCmd: "ip addr del 192.168.99.2/24 dev {{.}}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			hostsConfig := newHostsFileConfig(t, "127.0.0.1 localhost\n192.168.99.2 kaweezle.local\n")
			mockHost := host.NewMockHost(t)
			mockHost.On("GetHostsConfig").Return(hostsConfig).Once()
			mockHost.On("ExecPipe", mock.Anything, "ip -br -4 a sh").
				Return(script.Echo("eth0             UP             192.168.99.2/24\n")).Once()
			mockHost.On("ExecForEach", mock.Anything, tt.execForEachCmd).
				Return(script.NewPipe()).Once()

			config := &v1alpha1.IkniteClusterSpec{CreateIp: true, DomainName: "kaweezle.local"}
			err := k8s.ResetIPAddress(mockHost, config, tt.isDryRun)
			req.NoError(err)
		})
	}
}

func TestResetIPAddress_ExecForEachError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	hostsConfig := newHostsFileConfig(t, "127.0.0.1 localhost\n192.168.99.2 kaweezle.local\n")
	mockHost := host.NewMockHost(t)
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()
	mockHost.On("ExecPipe", mock.Anything, "ip -br -4 a sh").
		Return(script.Echo("eth0             UP             192.168.99.2/24\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "ip addr del 192.168.99.2/24 dev {{.}}").
		Return(script.NewPipe().WithError(errors.New("exec error"))).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "kaweezle.local",
	}

	err := k8s.ResetIPAddress(mockHost, config, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to delete IP address")
}

// --- ResetIPTables ---

func TestResetIPTables_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// No calls expected in dry run mode

	err := k8s.ResetIPTables(mockExec, true)
	req.NoError(err)
}

func TestResetIPTables_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// iptables chain
	mockExec.On("ExecPipe", mock.Anything, "iptables-save").
		Return(script.Echo("iptables content\n")).Once()
	mockExec.On("ExecPipe", mock.Anything, "iptables-restore").
		Return(script.NewPipe()).Once()
	// ip6tables chain
	mockExec.On("ExecPipe", mock.Anything, "ip6tables-save").
		Return(script.Echo("ip6tables content\n")).Once()
	mockExec.On("ExecPipe", mock.Anything, "ip6tables-restore").
		Return(script.NewPipe()).Once()

	err := k8s.ResetIPTables(mockExec, false)
	req.NoError(err)
}

func TestResetIPTables_IPTablesError(t *testing.T) { //nolint:dupl // same structure, different function under test
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "iptables-save").
		Return(script.Echo("content\n")).Once()
	mockExec.On("ExecPipe", mock.Anything, "iptables-restore").
		Return(script.NewPipe().WithError(errors.New("iptables error"))).Once()

	err := k8s.ResetIPTables(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to clean up iptables rules")
}

func TestResetIPTables_IP6TablesError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// iptables succeeds
	mockExec.On("ExecPipe", mock.Anything, "iptables-save").
		Return(script.Echo("content\n")).Once()
	mockExec.On("ExecPipe", mock.Anything, "iptables-restore").
		Return(script.NewPipe()).Once()
	// ip6tables fails
	mockExec.On("ExecPipe", mock.Anything, "ip6tables-save").
		Return(script.Echo("content\n")).Once()
	mockExec.On("ExecPipe", mock.Anything, "ip6tables-restore").
		Return(script.NewPipe().WithError(errors.New("ip6tables error"))).Once()

	err := k8s.ResetIPTables(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to clean up ip6tables rules")
}

// --- RemoveKubeletFiles ---

func TestRemoveKubeletFiles_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// No calls expected

	err := k8s.RemoveKubeletFiles(mockExec, true)
	req.NoError(err)
}

func TestRemoveKubeletFiles_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, mock.Anything).
		Return(script.NewPipe()).Once()

	err := k8s.RemoveKubeletFiles(mockExec, false)
	req.NoError(err)
}

func TestRemoveKubeletFiles_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, mock.Anything).
		Return(script.NewPipe().WithError(errors.New("rm failed"))).Once()

	err := k8s.RemoveKubeletFiles(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to remove kubelet files")
}

// --- StopAllContainers ---

func TestStopAllContainers_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// No calls expected

	err := k8s.StopAllContainers(mockExec, true)
	req.NoError(err)
}

func TestStopAllContainers_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, mock.Anything).
		Return(script.NewPipe()).Once()

	err := k8s.StopAllContainers(mockExec, false)
	req.NoError(err)
}

func TestStopAllContainers_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, mock.Anything).
		Return(script.NewPipe().WithError(errors.New("crictl failed"))).Once()

	err := k8s.StopAllContainers(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to stop all containers")
}

// --- DeleteCniNamespaces ---

func TestDeleteCniNamespaces_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	// ip netns show is still called in dry run (to list namespaces), but with "echo " prefix
	mockExec.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("ns1 ns2\nns3\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "echo ip netns delete {{.}}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteCniNamespaces(mockExec, true)
	req.NoError(err)
}

func TestDeleteCniNamespaces_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("ns1\nns2\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "ip netns delete {{.}}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteCniNamespaces(mockExec, false)
	req.NoError(err)
}

func TestDeleteCniNamespaces_Error(t *testing.T) { //nolint:dupl // same structure, different function under test
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("ns1\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "ip netns delete {{.}}").
		Return(script.NewPipe().WithError(errors.New("netns delete failed"))).Once()

	err := k8s.DeleteCniNamespaces(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to delete CNI namespaces")
}

// --- DeleteNetworkInterfaces ---

func TestDeleteNetworkInterfaces_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo("[]\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteNetworkInterfaces(mockExec, true)
	req.NoError(err)
}

func TestDeleteNetworkInterfaces_Success(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo("[]\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteNetworkInterfaces(mockExec, false)
	req.NoError(err)
}

func TestDeleteNetworkInterfaces_Error(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo("[]\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe().WithError(errors.New("link delete failed"))).Once()

	err := k8s.DeleteNetworkInterfaces(mockExec, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to delete network interfaces")
}

// --- UnmountPaths ---

// setupAllEvalSymlinksNotExist sets all EvalSymlinks calls to return os.ErrNotExist.
func setupAllEvalSymlinksNotExist(m *host.MockHost) {
	m.On("EvalSymlinks", mock.Anything).Return("", os.ErrNotExist)
}

func TestUnmountPaths_AllNotExist(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	setupAllEvalSymlinksNotExist(mockHost)

	err := k8s.UnmountPaths(mockHost, false, false)
	req.NoError(err)
}

func TestUnmountPaths_EvalSymlinksOtherError_ContinueOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// First path returns a non-NotExist error
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/pods").Return("", errors.New("perm denied")).Once()
	// Remaining paths return NotExist
	mockHost.On("EvalSymlinks", mock.Anything).Return("", os.ErrNotExist)

	err := k8s.UnmountPaths(mockHost, false /* failOnError */, false)
	req.NoError(err)
}

func TestUnmountPaths_EvalSymlinksOtherError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/pods").Return("", errors.New("perm denied")).Once()

	err := k8s.UnmountPaths(mockHost, true /* failOnError */, false)
	req.Error(err)
}

const dataManifestTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{ .name }}
  namespace: kube-system
spec:
  containers:
    - image: {{ .name }}:latest
      volumeMounts:
        - mountPath: {{ .APIBackendDatabaseDirectory }}
          name: {{ .name }}-data
        - mountPath: /etc/kubernetes/pki/{{ .name }}
          name: {{ .name }}-certs
  volumes:
    - hostPath:
        path: /etc/kubernetes/pki/{{ .name }}
        type: DirectoryOrCreate
      name: {{ .name }}-certs
    - hostPath:
        path: {{ .dataDir }}
        type: DirectoryOrCreate
      name: {{ .name }}-data
`

func createDataManifest(file io.Writer, backendName, dataDir string) error {
	manifestTemplate, err := template.New(backendName).Parse(dataManifestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse kine manifest template: %w", err)
	}

	config := map[string]string{
		"name":    backendName,
		"dataDir": dataDir,
	}
	if err := manifestTemplate.Execute(file, config); err != nil {
		return fmt.Errorf("failed to execute kine manifest template: %w", err)
	}
	return nil
}

func createDataManifestFile(fs host.FileSystem, backendName, dataDir, manifestDir string) error {
	manifestPath := filepath.Join(manifestDir, backendName+".yaml")
	file, err := fs.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create kine manifest file: %w", err)
	}
	defer file.Close() //nolint:errcheck // best-effort close in helper function

	if err := createDataManifest(file, backendName, dataDir); err != nil {
		return fmt.Errorf("failed to execute kine manifest template: %w", err)
	}

	return nil
}

func createManifestAndData(fs host.FileSystem, backendName, defaultDatadir, manifestDir string) error {
	err := createDataManifestFile(fs, backendName, defaultDatadir, manifestDir)
	if err != nil {
		return fmt.Errorf("failed to create manifest and data: %w", err)
	}
	err = fs.MkdirAll(defaultDatadir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	err = fs.WriteFile(filepath.Join(defaultDatadir, "db"), []byte("data"), 0o600)
	if err != nil {
		return fmt.Errorf("failed to create db file: %w", err)
	}
	return nil
}

func deleteAPIBackendDataNominalAssertions(
	req *require.Assertions,
	fs host.FileSystem,
	backendName, defaultDatadir, manifestDir string,
	shouldExist bool,
) {
	// Manifest should still exist
	exists, err := fs.Exists(filepath.Join(manifestDir, backendName+".yaml"))
	req.NoError(err)
	req.True(exists)
	// Data directory should still exist
	exists, err = fs.Exists(defaultDatadir)
	req.NoError(err)
	req.True(exists)
	// db file should no longer exist
	exists, err = fs.Exists(filepath.Join(defaultDatadir, "db"))
	req.NoError(err)
	req.Equal(shouldExist, exists)
}

func deleteAPIBackendDataNominalAssertionsDeleted(
	req *require.Assertions,
	fs host.FileSystem,
	backendName, defaultDatadir, manifestDir string,
) {
	deleteAPIBackendDataNominalAssertions(req, fs, backendName, defaultDatadir, manifestDir, false)
}

func deleteAPIBackendDataNominalAssertionsNotDeleted(
	req *require.Assertions,
	fs host.FileSystem,
	backendName, defaultDatadir, manifestDir string,
) {
	deleteAPIBackendDataNominalAssertions(req, fs, backendName, defaultDatadir, manifestDir, true)
}

// --- DeleteAPIBackendData ---.
func TestDeleteAPIBackendData(t *testing.T) {
	t.Parallel()
	standardManifestDir := filepath.Join(
		kubeadmConstants.KubernetesDir,
		kubeadmConstants.ManifestsSubDirName,
	)

	tests := []struct {
		createFS       func(t *testing.T, backendName, defaultDatadir, manifestDir string) host.FileSystem
		prepare        func(fs host.FileSystem, backendName, defaultDatadir, manifestDir string) error
		assertions     func(req *require.Assertions, fs host.FileSystem, backendName, defaultDatadir, manifestDir string)
		name           string
		backendName    string
		defaultDatadir string
		manifestDir    string
		wantErr        string
		dryRun         bool
	}{
		{
			name:           "dry run with non-existent manifest",
			dryRun:         true,
			backendName:    "nonexistent-backend",
			defaultDatadir: "/nonexistent/data",
			manifestDir:    "/nonexistent/manifests",
			prepare:        nil,
			wantErr:        "",
			assertions:     nil,
		},
		{
			name:           "dry run with existing manifest",
			dryRun:         true,
			backendName:    "kine",
			defaultDatadir: "/var/lib/kine",
			manifestDir:    standardManifestDir,
			prepare:        createManifestAndData,
			wantErr:        "",
			assertions:     deleteAPIBackendDataNominalAssertionsNotDeleted,
		},
		{
			name:           "bad manifest causes error",
			dryRun:         false,
			backendName:    "kine",
			defaultDatadir: "/var/lib/kine",
			manifestDir:    standardManifestDir,
			prepare: func(fs host.FileSystem, backendName, _, manifestDir string) error {
				// Create a manifest with invalid YAML
				invalidContent := []byte("invalid yaml: [")

				return fs.WriteFile(filepath.Join(manifestDir, backendName+".yaml"), invalidContent, 0o600)
			},
			wantErr: "failed to read API backend pod from disk",
		},
		{
			name:           "not dry run with existing manifest",
			dryRun:         false,
			backendName:    "kine",
			defaultDatadir: "/var/lib/kine",
			manifestDir:    standardManifestDir,
			prepare:        createManifestAndData,
			wantErr:        "",
			assertions:     deleteAPIBackendDataNominalAssertionsDeleted,
		},
		{
			name:           "not dry run with existing manifest but non existent data directory",
			dryRun:         false,
			backendName:    "kine",
			defaultDatadir: "/var/lib/kine",
			manifestDir:    standardManifestDir,
			prepare:        createDataManifestFile,
			wantErr:        "",
		},
		{
			name:           "not dry run with existing manifest and data directory but CleanDir fails",
			dryRun:         false,
			backendName:    "kine",
			defaultDatadir: "/var/lib/kine",
			manifestDir:    standardManifestDir,
			createFS: func(t *testing.T, backendName, defaultDatadir, _ string) host.FileSystem {
				t.Helper()

				mockFS := host.NewMockFileSystem(t)
				var manifest bytes.Buffer
				err := createDataManifest(&manifest, backendName, defaultDatadir)
				require.NoError(t, err)

				mockFS.On("ReadFile", mock.Anything).Return(manifest.Bytes(), nil).Once()
				mockFS.On("DirExists", defaultDatadir).Return(true, nil).Once()
				mockFS.On("ReadDir", defaultDatadir).Return([]os.FileInfo{}, fmt.Errorf("dir error")).Once()
				return mockFS
			},
			wantErr: "failed to delete API backend data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			var fs host.FileSystem
			if tt.createFS != nil {
				fs = tt.createFS(t, tt.backendName, tt.defaultDatadir, tt.manifestDir)
			} else {
				fs = host.NewMemMapFS()
			}

			if tt.prepare != nil {
				err := tt.prepare(fs, tt.backendName, tt.defaultDatadir, tt.manifestDir)
				req.NoError(err)
			}

			err := k8s.DeleteAPIBackendData(fs, tt.dryRun, tt.backendName, tt.defaultDatadir)
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
				if tt.assertions != nil {
					tt.assertions(req, fs, tt.backendName, tt.defaultDatadir, tt.manifestDir)
				}
			}
		})
	}
}

// --- CleanAll ---

func TestCleanAll_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers (dry run): no ExecPipe call
	// UnmountPaths: EvalSymlinks for all paths
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles (dry run): no ExecPipe call
	// DeleteCniNamespaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("")).Once()
	mockHost.On("ExecForEach", mock.Anything, "echo ip netns delete {{.}}").
		Return(script.NewPipe()).Once()
	// DeleteNetworkInterfaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo("[]\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, false, true /* isDryRun */)
	req.NoError(err)
}

func TestCleanAll_WithIPTablesAndIPAddress_DryRun(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	hostsConfig := newHostsFileConfig(t, "127.0.0.1 localhost\n")

	mockHost := host.NewMockHost(t)
	// StopAllContainers (dry run): no ExecPipe
	// UnmountPaths
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles (dry run): no ExecPipe
	// DeleteCniNamespaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("")).Once()
	mockHost.On("ExecForEach", mock.Anything, "echo ip netns delete {{.}}").
		Return(script.NewPipe()).Once()
	// DeleteNetworkInterfaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo("[]\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()
	// ResetIPTables (dry run): no calls
	// ResetIPAddress: host not mapped → just logs
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "nonexistent.local",
	}
	err := k8s.CleanAll(
		mockHost,
		config,
		true, /* resetIPAddress */
		true, /* resetIPTables */
		false,
		true, /* isDryRun */
	)
	req.NoError(err)
}

// --- ResetIPAddress (additional branches) ---

func TestResetIPAddress_HostsCreateError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	tmpDir := t.TempDir()
	// Create a file with no read permissions so txeh.NewHosts fails to read it.
	unreadable := filepath.Join(tmpDir, "hosts")
	require.NoError(t, os.WriteFile(unreadable, []byte("127.0.0.1 localhost\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) }) //nolint:errcheck // best-effort cleanup

	hostsConfig := &txeh.HostsConfig{
		ReadFilePath:  unreadable,
		WriteFilePath: unreadable,
	}

	mockHost := host.NewMockHost(t)
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "test.local",
	}

	err := k8s.ResetIPAddress(mockHost, config, false)
	req.Error(err)
	req.Contains(err.Error(), "failed to create hosts file handler")
}

func TestResetIPAddress_SaveError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	hostsConfig := newHostsFileConfig(t, "127.0.0.1 localhost\n192.168.99.2 kaweezle.local\n")
	// Override WriteFilePath to a non-writable location
	hostsConfig.WriteFilePath = filepath.Join(t.TempDir(), "nonexistent-dir", "hosts")

	mockHost := host.NewMockHost(t)
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()
	mockHost.On("ExecPipe", mock.Anything, "ip -br -4 a sh").
		Return(script.Echo("eth0             UP             192.168.99.2/24\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "ip addr del 192.168.99.2/24 dev {{.}}").
		Return(script.NewPipe()).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "kaweezle.local",
	}

	err := k8s.ResetIPAddress(mockHost, config, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to save hosts file")
}

// --- DeleteNetworkInterfaces (additional coverage: FilterLine body) ---

// interfaceJSON is a valid JSON array with one interface that matches the JQ filter (cni0).
const interfaceJSON = `[{"ifname":"cni0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"]}]`

func TestDeleteNetworkInterfaces_WithMatchingInterface(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo(interfaceJSON + "\n")).Once()
	// ExecForEach receives commands like "ip link delete cni0"
	mockExec.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteNetworkInterfaces(mockExec, false)
	req.NoError(err)
}

func TestDeleteNetworkInterfaces_DryRun_WithMatchingInterface(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockExec := host.NewMockExecutor(t)
	mockExec.On("ExecPipe", mock.Anything, "ip -j link show").
		Return(script.Echo(interfaceJSON + "\n")).Once()
	mockExec.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe()).Once()

	err := k8s.DeleteNetworkInterfaces(mockExec, true /* isDryRun */)
	req.NoError(err)
}

// --- UnmountPaths (additional: pathsToUnmountAndRemove error + failOnError) ---

func TestUnmountPaths_RemovePath_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// All pathsToUnmount paths return ErrNotExist (no error)
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/pods").Return("", os.ErrNotExist).Once()
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/plugins").Return("", os.ErrNotExist).Once()
	mockHost.On("EvalSymlinks", "/var/lib/kubelet").Return("", os.ErrNotExist).Once()
	// First pathsToUnmountAndRemove path returns a non-NotExist error
	mockHost.On("EvalSymlinks", "/run/containerd").Return("", errors.New("perm denied")).Once()

	err := k8s.UnmountPaths(mockHost, true /* failOnError */, false)
	req.Error(err)
}

func TestUnmountPaths_RemovePath_ContinueOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// All pathsToUnmount paths succeed (ErrNotExist → nil)
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/pods").Return("", os.ErrNotExist).Once()
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/plugins").Return("", os.ErrNotExist).Once()
	mockHost.On("EvalSymlinks", "/var/lib/kubelet").Return("", os.ErrNotExist).Once()
	// First pathsToUnmountAndRemove path fails, rest succeed (ErrNotExist)
	mockHost.On("EvalSymlinks", "/run/containerd").Return("", errors.New("perm denied")).Once()
	mockHost.On("EvalSymlinks", mock.Anything).Return("", os.ErrNotExist)

	err := k8s.UnmountPaths(mockHost, false /* failOnError */, false)
	req.NoError(err) // errors are logged but not returned
}

// --- DeleteAPIBackendData (additional: with manifest, CleanDir error) ---

// --- CleanAll (additional: error paths, failOnError branches) ---

// kubeletRemoveCmd matches the exact command used by RemoveKubeletFiles.
//
//nolint:lll // the command string is intentionally long to match the production constant exactly
const kubeletRemoveCmd = `sh -c 'rm -rf /var/lib/kubelet/{cpu_manager_state,memory_manager_state} /var/lib/kubelet/pods/*'`

// stopContainersCmd matches the exact command used by StopAllContainers.
const stopContainersCmd = "/bin/sh -c 'crictl rmp -f $(crictl pods -q)'"

func TestCleanAll_StopContainersError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).
		Return(script.NewPipe().WithError(errors.New("crictl failed"))).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to stop all containers")
}

func TestCleanAll_UnmountError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers succeeds (not dry run)
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).Return(script.NewPipe()).Once()
	// UnmountPaths: first path returns a non-NotExist error → with failOnError=true, returns error
	mockHost.On("EvalSymlinks", "/var/lib/kubelet/pods").Return("", errors.New("perm denied")).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to evaluate symlinks")
}

func TestCleanAll_RemoveKubeletError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers succeeds
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).Return(script.NewPipe()).Once()
	// UnmountPaths: all return ErrNotExist
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles fails
	mockHost.On("ExecPipe", mock.Anything, kubeletRemoveCmd).
		Return(script.NewPipe().WithError(errors.New("rm failed"))).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to remove kubelet files")
}

func TestCleanAll_DeleteCniError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers succeeds
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).Return(script.NewPipe()).Once()
	// UnmountPaths: all return ErrNotExist
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles succeeds
	mockHost.On("ExecPipe", mock.Anything, kubeletRemoveCmd).Return(script.NewPipe()).Once()
	// DeleteCniNamespaces fails
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").
		Return(script.Echo("ns1\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "ip netns delete {{.}}").
		Return(script.NewPipe().WithError(errors.New("netns failed"))).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to delete CNI namespaces")
}

func TestCleanAll_DeleteNetworkInterfacesError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers succeeds
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).Return(script.NewPipe()).Once()
	// UnmountPaths: all return ErrNotExist
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles succeeds
	mockHost.On("ExecPipe", mock.Anything, kubeletRemoveCmd).Return(script.NewPipe()).Once()
	// DeleteCniNamespaces succeeds
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").Return(script.Echo("")).Once()
	mockHost.On("ExecForEach", mock.Anything, "ip netns delete {{.}}").Return(script.NewPipe()).Once()
	// DeleteNetworkInterfaces fails
	mockHost.On("ExecPipe", mock.Anything, "ip -j link show").Return(script.Echo("[]\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "{{ . }}").
		Return(script.NewPipe().WithError(errors.New("link delete failed"))).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, false, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to delete network interfaces")
}

func TestCleanAll_IPTablesError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	mockHost := host.NewMockHost(t)
	// StopAllContainers succeeds (not dry run)
	mockHost.On("ExecPipe", mock.Anything, stopContainersCmd).Return(script.NewPipe()).Once()
	// UnmountPaths: all return ErrNotExist
	setupAllEvalSymlinksNotExist(mockHost)
	// RemoveKubeletFiles succeeds (not dry run)
	mockHost.On("ExecPipe", mock.Anything, kubeletRemoveCmd).Return(script.NewPipe()).Once()
	// DeleteCniNamespaces succeeds (not dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").Return(script.Echo("")).Once()
	mockHost.On("ExecForEach", mock.Anything, "ip netns delete {{.}}").Return(script.NewPipe()).Once()
	// DeleteNetworkInterfaces succeeds (not dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip -j link show").Return(script.Echo("[]\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "{{ . }}").Return(script.NewPipe()).Once()
	// ResetIPTables fails (not dry run)
	mockHost.On("ExecPipe", mock.Anything, "iptables-save").Return(script.Echo("content\n")).Once()
	mockHost.On("ExecPipe", mock.Anything, "iptables-restore").
		Return(script.NewPipe().WithError(errors.New("iptables error"))).Once()

	config := &v1alpha1.IkniteClusterSpec{CreateIp: false}
	err := k8s.CleanAll(mockHost, config, false, true /* resetIpTables */, true /* failOnError */, false /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to clean up iptables rules")
}

func TestCleanAll_IPAddressError_FailOnError(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")
	require.NoError(t, os.WriteFile(hostsFile, []byte("127.0.0.1 localhost\n192.168.99.2 kaweezle.local\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(hostsFile, 0o600) }) //nolint:errcheck // best-effort cleanup

	hostsConfig := &txeh.HostsConfig{
		ReadFilePath:  hostsFile,
		WriteFilePath: hostsFile,
	}

	mockHost := host.NewMockHost(t)
	// StopAllContainers (dry run): no ExecPipe
	// UnmountPaths: all return ErrNotExist
	setupAllEvalSymlinksNotExist(mockHost)
	// DeleteCniNamespaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip netns show").Return(script.Echo("")).Once()
	mockHost.On("ExecForEach", mock.Anything, "echo ip netns delete {{.}}").Return(script.NewPipe()).Once()
	// DeleteNetworkInterfaces (dry run)
	mockHost.On("ExecPipe", mock.Anything, "ip -j link show").Return(script.Echo("[]\n")).Once()
	mockHost.On("ExecForEach", mock.Anything, "{{ . }}").Return(script.NewPipe()).Once()
	// ResetIPAddress fails (hostsFile is unreadable)
	mockHost.On("GetHostsConfig").Return(hostsConfig).Once()

	config := &v1alpha1.IkniteClusterSpec{
		CreateIp:   true,
		DomainName: "kaweezle.local",
	}
	err := k8s.CleanAll(mockHost, config, true /* resetIpAddress */, false, true /* failOnError */, true /* isDryRun */)
	req.Error(err)
	req.Contains(err.Error(), "failed to create hosts file handler")
}
