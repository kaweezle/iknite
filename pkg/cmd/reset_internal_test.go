// cSpell: words paralleltest apimachinery kubeadmapiv1
//
//nolint:paralleltest // These tests modify global state and cannot be run in parallel
package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeadmapiv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/config"
)

const testCRISocket = "unix:///var/run/containerd/containerd.sock"

// makeResetCmd creates a cobra.Command with all reset flags, along with customized resetOptions.
// SkipCRIDetect=true and a known CRISocket are set so tests run without a real container runtime.
func makeResetCmd(opts *resetOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "reset"}
	AddResetFlags(cmd.Flags(), opts)
	config.AddIkniteClusterFlags(cmd.Flags(), opts.ikniteCfg)
	return cmd
}

func TestNewResetOptions(t *testing.T) {
	req := require.New(t)

	opts := newResetOptions()

	req.NotNil(opts)
	req.NotNil(opts.externalCfg)
	req.NotNil(opts.ikniteCfg)
	req.True(opts.externalCfg.Force, "Force should be true by default (iknite skips confirmation)")
	req.Equal(kubeadmConstants.GetAdminKubeConfigPath(), opts.kubeconfigPath,
		"kubeconfigPath should default to kubeadm's admin kubeconfig path")
	req.Equal([]string{"all"}, opts.ignorePreflightErrors,
		"All preflight errors should be ignored by default")
	req.Empty(opts.cfgPath, "cfgPath should be empty by default")
	req.False(opts.skipCRIDetect, "skipCRIDetect should be false by default")
}

func TestNewResetData(t *testing.T) {
	tests := []struct {
		customizeOptions func(t *testing.T, cmd *cobra.Command, opts *resetOptions) (func(), error)
		expectations     func(req *require.Assertions, data *resetData, output *bytes.Buffer)
		name             string
		wantErr          string
		args             []string
	}{
		{
			name: "default options",
			customizeOptions: func(_ *testing.T, _ *cobra.Command, opts *resetOptions) (func(), error) {
				opts.skipCRIDetect = true
				opts.externalCfg.CRISocket = testCRISocket
				return nil, nil //nolint:nilnil // no cleanup needed
			},
			expectations: func(req *require.Assertions, data *resetData, _ *bytes.Buffer) {
				req.NotNil(data)
				req.NotNil(data.Host())
				req.False(data.DryRun(), "dryRun should be false by default")
				req.True(data.ForceReset(), "force should be true by default in iknite")
				req.False(data.CleanupTmpDir(), "cleanupTmpDir should be false by default")
				req.Equal(sets.New("all"), data.IgnorePreflightErrors(),
					"All preflight errors should be ignored by default")
				req.Equal(testCRISocket, data.CRISocketPath())
				req.NotEmpty(data.CertificatesDir(), "CertificatesDir should not be empty")
				req.Equal(kubeadmapiv1.DefaultCertificatesDir, data.CertificatesDir(),
					"CertificatesDir should default to kubeadm's default cert dir")
				req.NotNil(data.IkniteCluster())
				req.NotNil(data.ResetCfg())
				req.Nil(data.Cfg(), "InitConfig should be nil when no cluster is available")
				req.Nil(data.Client(), "Client should be nil when no kubeconfig is available")
				req.NotNil(data.InputReader())
			},
		},
		{
			name: "dry run option",
			customizeOptions: func(t *testing.T, cmd *cobra.Command, opts *resetOptions) (func(), error) {
				t.Helper()
				opts.skipCRIDetect = true
				opts.externalCfg.CRISocket = testCRISocket
				require.NoError(t, cmd.Flags().Set(options.DryRun, "true"))
				return nil, nil //nolint:nilnil // no cleanup needed
			},
			expectations: func(req *require.Assertions, data *resetData, _ *bytes.Buffer) {
				req.NotNil(data)
				req.True(data.DryRun(), "dryRun should be true when the option is set")
				req.NotNil(data.Client(), "dry-run creates a fake client")
			},
		},
		{
			name: "force reset is true by default",
			customizeOptions: func(_ *testing.T, _ *cobra.Command, opts *resetOptions) (func(), error) {
				opts.skipCRIDetect = true
				opts.externalCfg.CRISocket = testCRISocket
				return nil, nil //nolint:nilnil // no cleanup needed
			},
			expectations: func(req *require.Assertions, data *resetData, _ *bytes.Buffer) {
				req.True(data.ForceReset())
			},
		},
		{
			name: "explicit certificates dir via flag",
			customizeOptions: func(t *testing.T, cmd *cobra.Command, opts *resetOptions) (func(), error) {
				t.Helper()
				opts.skipCRIDetect = true
				opts.externalCfg.CRISocket = testCRISocket
				certDir := filepath.Join(t.TempDir(), "pki")
				require.NoError(t, cmd.Flags().Set(options.CertificatesDir, certDir))
				return nil, nil //nolint:nilnil // no cleanup needed
			},
			expectations: func(req *require.Assertions, data *resetData, _ *bytes.Buffer) {
				req.NotNil(data)
				req.NotEmpty(data.CertificatesDir())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			opts := newResetOptions()
			var input, output bytes.Buffer

			cmd := makeResetCmd(opts)

			if tt.customizeOptions != nil {
				cleanup, err := tt.customizeOptions(t, cmd, opts)
				req.NoError(err)
				if cleanup != nil {
					defer cleanup()
				}
			}

			d, err := newResetData(cmd, opts, &input, &output, true)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
				req.NotNil(d)
				if tt.expectations != nil {
					tt.expectations(req, d, &output)
				}
			}
		})
	}
}

func TestResetDataGetters(t *testing.T) {
	t.Parallel()

	in := bytes.NewReader([]byte("input"))
	out := &bytes.Buffer{}
	ignoreErrors := sets.New("all")

	data := &resetData{
		certificatesDir:       "/etc/kubernetes/pki",
		criSocketPath:         testCRISocket,
		forceReset:            true,
		dryRun:                false,
		cleanupTmpDir:         true,
		inputReader:           in,
		outputWriter:          out,
		ignorePreflightErrors: ignoreErrors,
	}

	req := require.New(t)

	req.Equal("/etc/kubernetes/pki", data.CertificatesDir())
	req.Equal(testCRISocket, data.CRISocketPath())
	req.True(data.ForceReset())
	req.False(data.DryRun())
	req.True(data.CleanupTmpDir())
	req.Equal(in, data.InputReader())
	req.Equal(ignoreErrors, data.IgnorePreflightErrors())
	req.Nil(data.Client(), "client is nil when not set")
	req.Nil(data.Cfg(), "cfg is nil when not set")
	req.Nil(data.ResetCfg(), "resetCfg is nil when not set")
	req.Nil(data.IkniteCluster(), "ikniteCluster is nil when not set")
	req.Nil(data.Host(), "host is nil when not set")
}

func TestNewCmdReset(t *testing.T) {
	req := require.New(t)

	var in bytes.Buffer
	var out bytes.Buffer

	cmd := newCmdReset(&in, &out, nil)

	req.NotNil(cmd)
	req.Equal("reset", cmd.Name())
	req.NotEmpty(cmd.Short)

	// Verify essential flags are registered
	for _, flagName := range []string{
		options.DryRun,
		options.Force,
		options.NodeCRISocket,
		options.CertificatesDir,
	} {
		t.Run("has flag "+flagName, func(t *testing.T) {
			require.NotNil(t, cmd.Flags().Lookup(flagName), "flag %q should be registered", flagName)
		})
	}

	// Verify subcommands for phases are added by BindToCommand
	req.NotNil(cmd.Commands(), "reset command should have phase subcommands")
}

func TestNewCmdResetWithExplicitOptions(t *testing.T) {
	req := require.New(t)

	opts := newResetOptions()
	var in bytes.Buffer
	var out bytes.Buffer

	cmd := newCmdReset(&in, &out, opts)

	req.NotNil(cmd)
	req.Equal("reset", cmd.Name())
}

func TestResetDataWithKubeconfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()

	kubeconfigPath := filepath.Join(t.TempDir(), "custom.conf")
	kubeconfigContent := fmt.Appendf(nil, testKubeconfigDataFormat, ts.URL)
	if err := os.WriteFile(kubeconfigPath, kubeconfigContent, 0o600); err != nil {
		t.Fatalf("os.WriteFile returned unexpected error: %v", err)
	}

	opts := newResetOptions()
	opts.skipCRIDetect = true
	opts.externalCfg.CRISocket = testCRISocket
	opts.kubeconfigPath = kubeconfigPath

	var input, output bytes.Buffer
	cmd := makeResetCmd(opts)

	data, err := newResetData(cmd, opts, &input, &output, true)
	if err != nil {
		t.Fatalf("newResetData returned unexpected error: %v", err)
	}

	req := require.New(t)
	req.NotNil(data)
	req.NotNil(data.Client(), "client should be set when a valid kubeconfig is provided")
	req.Equal(testCRISocket, data.CRISocketPath())
}
