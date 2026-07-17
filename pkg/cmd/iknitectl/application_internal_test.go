/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package iknitectl

// cSpell: words dvcm appstage mockhost hostpkg crds

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	hostpkg "github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/testutil"
)

const (
	configMapContent = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`
)

func writeFile(t *testing.T, fileExecutor hostpkg.FileExecutor, path, content string) {
	t.Helper()
	if err := fileExecutor.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fileExecutor.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// stubFileInfo is a minimal os.FileInfo used by tests that need to fake a directory listing.
type stubFileInfo struct {
	name string
	dir  bool
}

func (f stubFileInfo) Name() string       { return f.name }
func (f stubFileInfo) Size() int64        { return 0 }
func (f stubFileInfo) Mode() os.FileMode  { return 0o644 }
func (f stubFileInfo) ModTime() time.Time { return time.Time{} }
func (f stubFileInfo) IsDir() bool        { return f.dir }
func (f stubFileInfo) Sys() any           { return nil }

func writeKustomizeApp(t *testing.T, fileExecutor hostpkg.FileExecutor, dir string) {
	t.Helper()

	writeFile(t, fileExecutor, filepath.Join(dir, "kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`)
	writeFile(t, fileExecutor, filepath.Join(dir, "configmap.yaml"), configMapContent)
}

func writeHelmChart(t *testing.T, fileExecutor hostpkg.FileExecutor, dir string) {
	t.Helper()

	writeFile(t, fileExecutor, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: app\nversion: 0.1.0\n")
}

func writeAppstageApplication(t *testing.T, fileExecutor hostpkg.FileExecutor, sourcePath string) {
	t.Helper()

	const appstageDir = "/repo/appstages/appstage-dev"
	kustomizationPath := filepath.Join(appstageDir, "kustomization.yaml")
	applicationPath := filepath.Join(appstageDir, "application.yaml")

	writeFile(
		t,
		fileExecutor,
		kustomizationPath,
		`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- application.yaml
`,
	)
	writeFile(
		t,
		fileExecutor,
		applicationPath,
		`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-one
spec:
  source:
    path: `+sourcePath+`
`,
	)
}

func newDummyFileExecutor(t *testing.T, options *testutil.DummyHostOptions) *testutil.DelegateHost {
	t.Helper()

	if options == nil {
		options = &testutil.DummyHostOptions{}
	}

	host, err := testutil.NewDummyHost(hostpkg.NewMemMapFS(), options)
	if err != nil {
		t.Fatalf("NewDummyHost: %v", err)
	}
	if networkHost, ok := host.Net.(*testutil.DummyNetworkHost); ok {
		t.Cleanup(func() {
			_ = networkHost.Cleanup() //nolint:errcheck // best effort cleanup in tests
		})
	}

	return host
}

// ---- CreateApplicationCmd ---------------------------------------------------

func TestCreateApplicationCmd(t *testing.T) {
	t.Parallel()

	cmd := CreateApplicationCmd(testutil.TestContainer(t).Scope("app"))
	if cmd == nil {
		t.Fatal("CreateApplicationCmd returned nil")
	}
	if cmd.Use != "application" {
		t.Errorf("expected Use 'application', got %s", cmd.Use)
	}
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, expected := range []string{"validate", "render", "render-all"} {
		if !names[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

func TestCreateApplicationCmd_ExecuteSubcommands(t *testing.T) {
	t.Parallel()

	t.Run("validate", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{
			FakeOutputs: map[string]*testutil.FakeProcessOutput{
				"^kubeconform .* -summary$": testutil.FakeExec("", 0),
			},
		})
		writeKustomizeApp(t, host, "/app")
		c := testutil.TestContainer(t)
		s := c.Scope("app")
		// Replace the host in the test container with our dummy host that simulates kubeconform output
		require.NoError(t, c.Decorate(func(_ *testutil.DelegateHost) *testutil.DelegateHost {
			return host
		}))

		cmd := CreateApplicationCmd(s)
		cmd.SetArgs([]string{"validate", "/app"})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}
	})

	t.Run("render with default host", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		tmpDir := "/base"
		s := testutil.TestContainer(t).Scope("app")

		fs := testutil.Resolve[hostpkg.FileSystem](t, s)

		req.NoError(fs.MkdirAll(tmpDir, os.FileMode(0o600)))
		req.NoError(fs.WriteFile(
			filepath.Join(tmpDir, "kustomization.yaml"),
			[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
			0o600,
		))

		req.NoError(fs.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600))

		cmd := CreateApplicationCmd(s)
		cmd.SetArgs([]string{"render", tmpDir})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}

		out := testutil.Resolve[*bytes.Buffer](t, s)
		if !bytes.Contains(out.Bytes(), []byte("ConfigMap")) {
			t.Fatalf("expected rendered output, got %s", out.String())
		}
	})

	t.Run("render-all", func(t *testing.T) {
		t.Parallel()

		host := newDummyFileExecutor(t, &testutil.DummyHostOptions{
			FakeOutputs: map[string]*testutil.FakeProcessOutput{
				"^helm template app-one /repo/apps/app-one --skip-crds$": testutil.FakeExec(configMapContent, 0),
			},
		})
		writeAppstageApplication(t, host, "apps/app-one")
		writeHelmChart(t, host, "/repo/apps/app-one")
		c := testutil.TestContainer(t)
		s := c.Scope("app")
		require.NoError(t, c.Decorate(func(_ *testutil.DelegateHost) *testutil.DelegateHost { return host }))

		cmd := CreateApplicationCmd(s)
		cmd.SetArgs([]string{"render-all", "/repo/appstages", "/dest", "--base-dir", "/repo"})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext: %v", err)
		}
		out := testutil.Resolve[*bytes.Buffer](t, s)
		if !strings.Contains(out.String(), "Rendering appstage appstage-dev") ||
			!strings.Contains(out.String(), "Rendering application app-one from apps/app-one") {
			t.Fatalf("unexpected output: %s", out.String())
		}
		files, err := host.ReadDir("/dest/appstage-dev/applications/app-one")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(files) != 1 || files[0].Name() != "ConfigMap-test-config.yaml" {
			t.Fatalf("unexpected rendered files: %+v", files)
		}
	})
}
