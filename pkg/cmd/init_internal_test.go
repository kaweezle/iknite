// cSpell: words paralleltest apimachinery
//
//nolint:paralleltest // These tests modify global state and cannot be run in parallel
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/options"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	ikniteApi "github.com/kaweezle/iknite/pkg/apis/iknite"
)

func TestNewInitData(t *testing.T) {
	tests := []struct {
		customizeOptions func(t *testing.T, cmd *cobra.Command, opts *initOptions) (func(), error)
		expectations     func(req *require.Assertions, data *initData, output *bytes.Buffer)
		name             string
		wantErr          string
		args             []string
	}{
		{
			name: "default options",
			expectations: func(req *require.Assertions, data *initData, _ *bytes.Buffer) {
				req.NotNil(data)
				req.NotNil(data.Host())
				req.False(data.DryRun(), "dryRun should be false by default")
				req.False(data.UploadCerts(), "By default, certs should not be uploaded for kubeadm init")
				req.Equal(
					data.IgnorePreflightErrors(),
					sets.New("all"),
					"By default, no preflight errors should be ignored",
				)
				req.Equal(
					filepath.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.DefaultCertificateDir),
					data.CertificateWriteDir(),
					"CertificateWriteDir should be set to kubeadm's default cert dir by default",
				)
				req.Equal(
					kubeadmConstants.KubernetesDir,
					data.KubeConfigDir(),
					"KubeConfigDir should be set to kubeadm's default kubeconfig dir by default",
				)
				req.Equal(
					filepath.Join(kubeadmConstants.KubernetesDir, kubeadmConstants.AdminKubeConfigFileName),
					data.KubeConfigPath(),
					"KubeConfigPath should be set to kubeadm's default admin kubeconfig file by default",
				)
				req.Equal(
					kubeadmConstants.GetStaticPodDirectory(),
					data.ManifestDir(),
					"ManifestDir should be set to kubeadm's default manifest dir by default",
				)
				req.Equal(
					kubeadmConstants.KubeletRunDirectory,
					data.KubeletDir(),
					"KubeletDir should be set to kubeadm's default kubelet dir by default",
				)
			},
		},
		{
			name: "dry run option",
			customizeOptions: func(t *testing.T, cmd *cobra.Command, _ *initOptions) (func(), error) {
				t.Helper()
				// Need to change dry run through flags for it to be picked up by newInitData.
				flags := cmd.Flags()
				require.NoError(t, flags.Set(options.DryRun, "true"))
				dir := t.TempDir()
				dryRunDir := filepath.Join(dir, "dry-run")
				oldEnv := os.Getenv("KUBEADM_INIT_DRYRUN_DIR")
				os.Setenv("KUBEADM_INIT_DRYRUN_DIR", dryRunDir) //nolint:errcheck // assume i's ok
				return func() {
					os.Setenv("KUBEADM_INIT_DRYRUN_DIR", oldEnv) //nolint:errcheck // assume i's ok
				}, nil
			},
			expectations: func(req *require.Assertions, data *initData, _ *bytes.Buffer) {
				req.NotNil(data)
				req.NotEmpty(data.dryRunDir)
				req.True(data.DryRun(), "dryRun should be true when the option is set")
				req.Contains(
					data.CertificateWriteDir(),
					os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
					"CertificateWriteDir should be inside the dry run dir when dry run is enabled",
				)
				req.Contains(
					data.KubeConfigPath(),
					os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
					"kubeconfigPath should be inside the dry run dir when dry run is enabled",
				)
				req.Contains(
					data.KubeConfigDir(),
					os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
					"KubeConfigDir should be inside the dry run dir when dry run is enabled",
				)
				req.Contains(
					data.ManifestDir(),
					os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
					"ManifestDir should be inside the dry run dir when dry run is enabled",
				)
				req.Contains(
					data.KubeletDir(),
					os.Getenv("KUBEADM_INIT_DRYRUN_DIR"),
					"KubeletDir should be inside the dry run dir when dry run is enabled",
				)
			},
		},
		{
			name: "dry run with error creating dry run dir",
			customizeOptions: func(t *testing.T, cmd *cobra.Command, _ *initOptions) (func(), error) {
				t.Helper()
				// Need to change dry run through flags for it to be picked up by newInitData.
				flags := cmd.Flags()
				require.NoError(t, flags.Set(options.DryRun, "true"))
				return nil, nil //nolint:nilnil // no cleanup needed
			},
			wantErr: "couldn't create a temporary directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			opts := newInitOptions()
			initRunner := workflow.NewRunner()
			var output bytes.Buffer // dummy output for validation

			cmd := newCmdInit(&output, opts, initRunner)
			if tt.customizeOptions != nil {
				cleanup, err := tt.customizeOptions(t, cmd, opts)
				req.NoError(err)
				if cleanup != nil {
					defer cleanup()
				}
			}

			d, err := initRunner.InitData(tt.args)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
				data, ok := d.(*initData)
				req.True(ok, "InitData should be of type *initData")
				if tt.expectations != nil {
					tt.expectations(req, data, &output)
				}
			}
		})
	}
}

const testKubeconfigDataFormat = `---
apiVersion: v1
clusters:
- name: foo-cluster
  cluster:
    server: %s
contexts:
- name: foo-context
  context:
    cluster: foo-cluster
current-context: foo-context
kind: Config
`

func TestInitDataClientWithNonDefaultKubeconfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()

	kubeconfigPath := filepath.Join(t.TempDir(), "custom.conf")
	if err := os.WriteFile(kubeconfigPath, fmt.Appendf(nil, testKubeconfigDataFormat, ts.URL), 0o600); err != nil {
		t.Fatalf("os.WriteFile returned unexpected error: %v", err)
	}

	// initialize an external init option and inject it to the init cmd
	initOptions := newInitOptions()
	initOptions.skipCRIDetect = true // avoid CRI detection in unit tests
	initOptions.kubeconfigPath = kubeconfigPath
	initRunner := workflow.NewRunner()
	var output bytes.Buffer // dummy output for validation
	_ = newCmdInit(&output, initOptions, initRunner)

	d, err := initRunner.InitData([]string{})
	if err != nil {
		t.Fatalf("initRunner.InitData returned unexpected error: %v", err)
	}
	data, ok := d.(*initData)
	if !ok {
		t.Fatalf("InitData should be of type *initData")
	}

	client, err := data.Client()
	if err != nil {
		t.Fatalf("data.Client returned unexpected error: %v", err)
	}

	_, err = data.WaitControlPlaneClient()
	if err != nil {
		t.Fatalf("data.WaitControlPlaneClient returned unexpected error: %v", err)
	}

	_, err = data.ClientWithoutBootstrap()
	if err != nil {
		t.Fatalf("data.ClientWithoutBootstrap returned unexpected error: %v", err)
	}

	path := data.KubeConfigPath()
	if path != kubeconfigPath {
		t.Fatalf("data.KubeconfigPath returned unexpected path: got %s, want %s", path, kubeconfigPath)
	}
	_, err = data.KubeConfig()
	if err != nil {
		t.Fatalf("data.KubeConfig returned unexpected error: %v", err)
	}
	config, err := data.KubeConfig()
	if err != nil {
		t.Fatalf("data.KubeConfig returned unexpected error: %v", err)
	}
	// config and testKubeconfigDataFormat should be structurally the same.
	if config.Clusters["foo-cluster"].Server != ts.URL {
		t.Fatalf("data.KubeConfig returned unexpected cluster server URL: got %s, want %s",
			config.Clusters["foo-cluster"].Server, ts.URL)
	}

	result := client.Discovery().RESTClient().Verb("HEAD").Do(context.Background())
	if err = result.Error(); err != nil {
		t.Fatalf("REST client request returned unexpected error: %v", err)
	}

	idx := slices.IndexFunc(initRunner.Phases, func(p workflow.Phase) bool { return p.Name == "preflight" })
	if idx == -1 {
		t.Fatalf("preflight phase not found in initRunner.Phases")
	}
	preflightPhase := initRunner.Phases[idx]
	err = preflightPhase.Run(d)
	if err != nil {
		t.Fatalf("preflight phase run returned unexpected error: %v", err)
	}
	status := data.IkniteCluster().Status
	require.Equal(
		t,
		ikniteApi.Initializing,
		status.State,
		"Cluster state should be 'Initializing' after preflight phase",
	)
	require.Equal(t, "preflight", status.CurrentPhase, "Current phase should be preflight")
}
