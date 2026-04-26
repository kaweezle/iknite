// cSpell: words testutil clientcmd readyz clusterroles
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kaweezle/iknite/pkg/constants"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
	"github.com/kaweezle/iknite/pkg/utils"
)

const baseKustomizationDir = "/base"

func Test_performKustomize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		//nolint:lll // test case struct, ignore line length
		prepare      func(t *testing.T, fs host.FileSystem, kOpts *utils.KustomizeOptions, wOpts *utils.WaitOptions, sOpts *testutil.TestServerOptions) error
		expectations func(req *require.Assertions, fs host.FileSystem, logs []testutil.RequestLog)
		name         string
		wantErr      string
	}{
		{
			name: "no kustomization file, should use embedded configuration",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				_ *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				wOpts.Timeout = 5 * time.Second
				return nil
			},
			expectations: func(req *require.Assertions, _ host.FileSystem, logs []testutil.RequestLog) {
				req.GreaterOrEqual(len(logs), 2)
				idx := slices.IndexFunc(logs, func(p testutil.RequestLog) bool { return p.Method == http.MethodPost })
				req.GreaterOrEqual(idx, 0, "Expected a POST request to apply the kustomize configuration")
				log := logs[idx]
				req.Equal("/api/v1/namespaces/kube-system/configmaps", log.Path)
			},
		},
		{
			name: "Basic flow with kustomization file, should apply configuration successfully",
			prepare: func(
				t *testing.T,
				fs host.FileSystem,
				kOpts *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				wOpts.Timeout = 5 * time.Second
				return nil
			},
			expectations: func(req *require.Assertions, _ host.FileSystem, logs []testutil.RequestLog) {
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
				_ *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				wOpts.Timeout = 5 * time.Second

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
				_ *utils.KustomizeOptions,
				wOpts *utils.WaitOptions,
				sOpts *testutil.TestServerOptions,
			) error {
				t.Helper()
				sOpts.FailurePaths = []string{"/readyz"}
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				wOpts.Timeout = 5 * time.Second
				return nil
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
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				sOpts.FailurePaths = []string{"/api/v1/namespaces/kube-system/configmaps/test-config"}
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				wOpts.Timeout = 5 * time.Second
				return nil
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
				if err := testutil.CreateBasicKustomization(fs, baseKustomizationDir); err != nil {
					return fmt.Errorf("failed to create basic kustomization: %w", err)
				}
				kOpts.Kustomization = baseKustomizationDir
				config := testutil.CreateTestAPIServer(t, testutil.ContentPatchHandler("with_resources", sOpts))
				testutil.WriteRestConfigToFile(t, config, fs, constants.KubernetesRootConfig, "iknite")
				wOpts.Timeout = 5 * time.Second
				wOpts.Wait = true
				wOpts.Watch = true
				return nil
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
					tt.expectations(req, fs, srvOptions.Requests)
				}
			}
		})
	}
}
