// cSpell: words testutil clientcmd readyz clusterroles paralleltest
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
	"github.com/kaweezle/iknite/pkg/utils"
)

const baseKustomizationDir = "/base"

type KustomizeTestCase struct {
	name string
	//nolint:lll // test case struct, ignore line length
	prepare      func(t *testing.T, fs host.FileSystem, kOpts *utils.KustomizeOptions, wOpts *utils.WaitOptions, sOpts *testutil.TestServerOptions) error
	prepareCmd   func(t *testing.T, cmd *cobra.Command) error
	expectations func(req *require.Assertions, fs host.FileSystem, logs []testutil.RequestLog, output *bytes.Buffer)
	wantErr      string
}

func standardPrepareKustomization(
	t *testing.T,
	fs host.FileSystem,
	_ *utils.KustomizeOptions,
	wOpts *utils.WaitOptions,
	sOpts *testutil.TestServerOptions,
) error {
	t.Helper()
	config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
	testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite", "static")
	wOpts.Timeout = 5 * time.Second
	return nil
}

func basicPrepareKustomization(
	t *testing.T,
	fs host.FileSystem,
	kOpts *utils.KustomizeOptions,
	wOpts *utils.WaitOptions,
	sOpts *testutil.TestServerOptions,
) error {
	t.Helper()
	if err := standardPrepareKustomization(t, fs, kOpts, wOpts, sOpts); err != nil {
		return fmt.Errorf("failed to prepare kustomization: %w", err)
	}

	if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir, false); err != nil {
		return fmt.Errorf("failed to create basic kustomization: %w", err)
	}
	kOpts.Kustomization = baseKustomizationDir
	return nil
}

var nominalTestCase = &KustomizeTestCase{
	name:    "no kustomization file, should use embedded configuration",
	prepare: standardPrepareKustomization,
	expectations: func(req *require.Assertions, _ host.FileSystem, logs []testutil.RequestLog, _ *bytes.Buffer) {
		req.GreaterOrEqual(len(logs), 2)
		idx := slices.IndexFunc(logs, func(p testutil.RequestLog) bool { return p.Method == http.MethodPost })
		req.GreaterOrEqual(idx, 0, "Expected a POST request to apply the kustomize configuration")
		log := logs[idx]
		req.Equal("/api/v1/namespaces/kube-system/configmaps", log.Path)
	},
}

func Test_performKustomize(t *testing.T) {
	t.Parallel()
	tests := []*KustomizeTestCase{
		nominalTestCase,
		{
			name:    "Basic flow with kustomization file, should apply configuration successfully",
			prepare: basicPrepareKustomization,
			expectations: func(req *require.Assertions, _ host.FileSystem, logs []testutil.RequestLog,
				_ *bytes.Buffer,
			) {
				req.GreaterOrEqual(len(logs), 2)
				idx := slices.IndexFunc(logs, func(p testutil.RequestLog) bool { return p.Method == http.MethodPost })
				req.GreaterOrEqual(idx, 0, "Expected a POST request to apply the kustomize configuration")
				log := logs[idx]
				req.Equal("/api/v1/namespaces/kube-system/configmaps", log.Path)
			},
		},
		{
			name:    "no client configuration, should return error",
			wantErr: "while loading local cluster configuration",
		},
		{
			name:    "Fail to create rest client from configuration",
			wantErr: "failed to create REST client",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				err := standardPrepareKustomization(t, fs, kOpts, wOpts, sOpts)
				if err != nil {
					return fmt.Errorf("failed to prepare kustomization: %w", err)
				}

				content, err := fs.ReadFile(constants.KubernetesRootConfig)
				if err != nil {
					return fmt.Errorf("failed to read kubeconfig file: %w", err)
				}
				apiConfig, err := clientcmd.Load(content)
				if err != nil {
					return fmt.Errorf("failed to parse kubeconfig file: %w", err)
				}
				apiConfig.CurrentContext = "nonexistent-context"

				content, err = clientcmd.Write(*apiConfig)
				if err != nil {
					return fmt.Errorf("failed to write kubeconfig file: %w", err)
				}
				err = fs.WriteFile(constants.KubernetesRootConfig, content, 0o644)
				if err != nil {
					return fmt.Errorf("failed to write kubeconfig file: %w", err)
				}
				return nil
			},
		},
		{
			name: "api server not responding, should return error",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					"/readyz": testutil.FailOverrideHandler,
				}
				wOpts.Interval = 100 * time.Millisecond
				wOpts.Retries = 1
				return standardPrepareKustomization(t, fs, kOpts, wOpts, sOpts)
			},
			wantErr: "failed to check if cluster is running",
		},
		{
			name: "Fail to push kustomization",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir, false); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				sOpts.Overrides = map[string]testutil.HandlerOverrideFunc{
					"/api/v1/namespaces/kube-system/configmaps/test-config": testutil.FailOverrideHandler,
				}
				return standardPrepareKustomization(t, fs, kOpts, wOpts, sOpts)
			},
			wantErr: "failed to apply kustomize configuration",
		},
		{
			name: "Fail to push kustomization",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir, false); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				wOpts.Wait = true
				wOpts.Watch = true
				return standardPrepareKustomization(t, fs, kOpts, wOpts, sOpts)
			},
			wantErr: "failed to wait for workloads",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)
			fs := host.NewMemMapFS()
			kustomizationOptions := utils.NewKustomizeOptions()
			waitOptions := utils.NewWaitOptions()
			srvOptions := &testutil.TestServerOptions{}
			if tt.prepare != nil {
				req.NoError(tt.prepare(t, fs, kustomizationOptions, waitOptions, srvOptions))
			}
			gotErr := performKustomize(context.Background(), fs, kustomizationOptions, waitOptions)
			if tt.wantErr != "" {
				req.Error(gotErr)
				req.Contains(gotErr.Error(), tt.wantErr)
			} else {
				req.NoError(gotErr)
				if tt.expectations != nil {
					tt.expectations(req, fs, srvOptions.Requests, nil)
				}
			}
		})
	}
}

type errorWriter struct{}

func (e *errorWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

//nolint:paralleltest // Kustomize is not parallel friendly
func TestKustomizeCmd(t *testing.T) {
	tests := []*KustomizeTestCase{
		nominalTestCase,
		{
			name:    "no client configuration, should return error",
			wantErr: "while loading local cluster configuration",
		},
		{
			name:    "Print kustomization, should write resources to output",
			prepare: basicPrepareKustomization,
			prepareCmd: func(t *testing.T, cmd *cobra.Command) error {
				t.Helper()
				cmd.SetArgs([]string{"print"})
				return nil
			},
			expectations: func(req *require.Assertions, _ host.FileSystem,
				_ []testutil.RequestLog, out *bytes.Buffer,
			) {
				output := out.String()
				req.NotEmpty(output, "Expected output to contain the rendered kustomization resources")
				req.Contains(output, "kind: ConfigMap")
				req.Contains(output, "name: test-config")
			},
		},
		{
			name: "Failing kustomization should return error",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				_ *utils.WaitOptions,
				_ *testutil.TestServerOptions,
			) error {
				t.Helper()
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir, true); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				return nil
			},
			prepareCmd: func(t *testing.T, cmd *cobra.Command) error {
				t.Helper()
				cmd.SetArgs([]string{"print"})
				return nil
			},
			wantErr: "while getting kustomization resources",
		},
		{
			name: "Fail to output kustomization, should return error",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				_ *utils.WaitOptions,
				_ *testutil.TestServerOptions,
			) error {
				t.Helper()
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir, false); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				return nil
			},
			prepareCmd: func(t *testing.T, cmd *cobra.Command) error {
				t.Helper()
				cmd.SetArgs([]string{"print"})
				cmd.SetOut(&errorWriter{})
				return nil
			},
			wantErr: "while writing kustomization resources",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			fs := host.NewMemMapFS()
			kustomizationOptions := utils.NewKustomizeOptions()
			waitOptions := utils.NewWaitOptions()
			srvOptions := &testutil.TestServerOptions{}
			if tt.prepare != nil {
				req.NoError(tt.prepare(t, fs, kustomizationOptions, waitOptions, srvOptions))
			}
			cmd := NewKustomizeCmd(kustomizationOptions, waitOptions, fs)
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			if tt.prepareCmd != nil {
				req.NoError(tt.prepareCmd(t, cmd))
			}
			err := cmd.ExecuteContext(t.Context())
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
				if tt.expectations != nil {
					tt.expectations(req, fs, srvOptions.Requests, out)
				}
			}
		})
	}
}
