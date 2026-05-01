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
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"

	mockhost "github.com/kaweezle/iknite/mocks/pkg/host"
	hostpkg "github.com/kaweezle/iknite/pkg/host"
)

// ---- helpers ----------------------------------------------------------------

func newMemFileExecutor(t *testing.T) hostpkg.FileExecutor {
	t.Helper()

	fileExecutor, ok := hostpkg.NewMemMapFS().(hostpkg.FileExecutor)
	if !ok {
		t.Fatal("failed to create FileExecutor from mem fs")
	}

	return fileExecutor
}

func writeFile(t *testing.T, fileExecutor hostpkg.FileExecutor, path, content string) {
	t.Helper()
	if err := fileExecutor.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fileExecutor.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

type applicationContextKey string

func mustResMapFromYAML(t *testing.T, yamlData string) resmap.ResMap {
	t.Helper()

	resources, err := resmap.NewFactory(provider.NewDefaultDepProvider().GetResourceFactory()).
		NewResMapFromBytes([]byte(yamlData))
	if err != nil {
		t.Fatalf("NewResMapFromBytes: %v", err)
	}

	return resources
}

func assertRenderCommand(
	t *testing.T,
	run func(context.Context, hostpkg.FileExecutor) (resmap.ResMap, error),
	wantCmd string,
	wantArgs []string,
) {
	t.Helper()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), wantCmd)
	fileExecutor := mockhost.NewMockFileExecutor(t)
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
		func(actualCtx context.Context, options *hostpkg.CommandOptions) {
			if actualCtx != ctx {
				t.Fatalf("expected context to be passed through unchanged")
			}
			if options.Cmd != wantCmd {
				t.Fatalf("expected command %q, got %q", wantCmd, options.Cmd)
			}
			if !reflect.DeepEqual(options.Args, wantArgs) {
				t.Fatalf("expected args %v, got %v", wantArgs, options.Args)
			}
			if options.Stdout == nil {
				t.Fatal("expected stdout writer to be configured")
			}
			if _, err := io.WriteString(options.Stdout, configMapContent); err != nil {
				t.Fatalf("WriteString: %v", err)
			}
		},
	).Return(nil)

	resources, err := run(ctx, fileExecutor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	yamlData, err := resources.AsYaml()
	if err != nil {
		t.Fatalf("AsYaml: %v", err)
	}
	if !bytes.Contains(yamlData, []byte("kind: ConfigMap")) {
		t.Fatalf("expected ConfigMap in output, got: %s", string(yamlData))
	}
}

// ---- detectAppType ----------------------------------------------------------

func TestDetectAppType_Kustomize(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/kustomization.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeKustomize {
		t.Errorf("expected appTypeKustomize, got %v", typ)
	}
	if path != "/app" {
		t.Errorf("expected path /app, got %s", path)
	}
}

func TestDetectAppType_Helmfile(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/helmfile.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelmfile {
		t.Errorf("expected appTypeHelmfile, got %v", typ)
	}
	if path != "/app/helmfile.yaml" {
		t.Errorf("expected path /app/helmfile.yaml, got %s", path)
	}
}

func TestDetectAppType_HelmfileGotmpl(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/helmfile.yaml.gotmpl", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelmfile {
		t.Errorf("expected appTypeHelmfile, got %v", typ)
	}
	if path != "/app/helmfile.yaml.gotmpl" {
		t.Errorf("expected path /app/helmfile.yaml.gotmpl, got %s", path)
	}
}

func TestDetectAppType_Helm(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	writeFile(t, fileExecutor, "/app/Chart.yaml", "")

	typ, path, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeHelm {
		t.Errorf("expected appTypeHelm, got %v", typ)
	}
	if path != "/app" {
		t.Errorf("expected path /app, got %s", path)
	}
}

func TestDetectAppType_Unknown(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/app", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	typ, _, err := detectAppType(fileExecutor, "/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != appTypeUnknown {
		t.Errorf("expected appTypeUnknown, got %v", typ)
	}
}

// ---- CreateApplicationCmd ---------------------------------------------------

func TestCreateApplicationCmd(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := CreateApplicationCmd(newMemFileExecutor(t), out)
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

// ---- runValidateApp / runRenderApp error paths ------------------------------

func TestRunValidateApp_MissingDir(t *testing.T) {
	t.Parallel()
	err := runValidateApp(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_MissingDir(t *testing.T) {
	t.Parallel()
	err := runRenderApp(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_UnknownType(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/app", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderApp(context.Background(), fileExecutor, &bytes.Buffer{}, "/app", "")
	if err == nil {
		t.Error("expected error for directory with no recognized app definition")
	}
}

func TestRenderHelmfile_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	assertRenderCommand(
		t,
		func(ctx context.Context, fileExecutor hostpkg.FileExecutor) (resmap.ResMap, error) {
			return renderHelmfile(ctx, fileExecutor, "/app/helmfile.yaml")
		},
		"helmfile",
		[]string{"template", "--skip-tests", "--args=--skip-crds", "-f", "/app/helmfile.yaml"},
	)
}

func TestRenderHelm_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	assertRenderCommand(
		t,
		func(ctx context.Context, fileExecutor hostpkg.FileExecutor) (resmap.ResMap, error) {
			return renderHelm(ctx, fileExecutor, "/charts/example")
		},
		"helm",
		[]string{"template", "example", "/charts/example", "--skip-crds"},
	)
}

func TestRunKubeconform_UsesExecutorAndContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), applicationContextKey("name"), "kubeconform")
	fileExecutor := mockhost.NewMockFileExecutor(t)
	expectedArgs := []string{
		"-schema-location",
		"default",
		"-schema-location",
		"/schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/" +
			"{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/master/customresourcedefinition.json",
		"-summary",
	}
	fileExecutor.EXPECT().RunCommand(ctx, mock.Anything).Run(
		func(actualCtx context.Context, options *hostpkg.CommandOptions) {
			if actualCtx != ctx {
				t.Fatalf("expected context to be passed through unchanged")
			}
			if options.Cmd != "kubeconform" {
				t.Fatalf("expected kubeconform command, got %q", options.Cmd)
			}
			if !reflect.DeepEqual(options.Args, expectedArgs) {
				t.Fatalf("expected args %v, got %v", expectedArgs, options.Args)
			}
			stdinData, err := io.ReadAll(options.Stdin)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if !bytes.Contains(stdinData, []byte("kind: ConfigMap")) {
				t.Fatalf("expected ConfigMap in stdin, got: %s", string(stdinData))
			}
			if options.Stdout != os.Stderr {
				t.Fatal("expected stdout to use os.Stderr")
			}
			if options.Stderr != os.Stderr {
				t.Fatal("expected stderr to use os.Stderr")
			}
		},
	).Return(nil)

	err := runKubeconform(ctx, fileExecutor, mustResMapFromYAML(t, configMapContent), "/schemas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- runRenderApp with kustomize (integration) ------------------------------

func TestRunRenderApp_Kustomize_Stdout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runRenderApp(context.Background(), hostpkg.NewDefaultHost(), &out, tmpDir, ""); err != nil {
		t.Fatalf("runRenderApp: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("ConfigMap")) {
		t.Errorf("expected ConfigMap in output, got: %s", out.String())
	}
}

func TestRunRenderApp_Kustomize_SplitFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "out")
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatal(err)
	}

	fileExecutor := hostpkg.NewDefaultHost()
	if err := runRenderApp(context.Background(), fileExecutor, &bytes.Buffer{}, tmpDir, destDir); err != nil {
		t.Fatalf("runRenderApp: %v", err)
	}

	files, err := fileExecutor.ReadDir(destDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0].Name() != "ConfigMap-test-config.yaml" {
		t.Errorf("unexpected filename: %s", files[0].Name())
	}
}

// ---- parseApplicationsFromDir -----------------------------------------------

func TestParseApplicationsFromDir(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)

	appYAML := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    path: deploy/k8s/argocd/e2e/my-app
`
	nonAppYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
`
	writeFile(t, fileExecutor, "/manifests/Application-my-app.yaml", appYAML)
	writeFile(t, fileExecutor, "/manifests/ConfigMap-cfg.yaml", nonAppYAML)

	apps, err := parseApplicationsFromDir(fileExecutor, "/manifests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Metadata.Name != "my-app" {
		t.Errorf("expected name 'my-app', got '%s'", apps[0].Metadata.Name)
	}
	if apps[0].Spec.Source.Path != "deploy/k8s/argocd/e2e/my-app" {
		t.Errorf("unexpected source path: %s", apps[0].Spec.Source.Path)
	}
}

// ---- runRenderAll error paths -----------------------------------------------

func TestRunRenderAll_MissingAppstagesDir(t *testing.T) {
	t.Parallel()
	err := runRenderAll(context.Background(), newMemFileExecutor(t), &bytes.Buffer{}, "/nonexistent", "/dest", ".")
	if err == nil {
		t.Error("expected error for nonexistent appstages directory")
	}
}

func TestRunRenderAll_NoAppstages(t *testing.T) {
	t.Parallel()
	fileExecutor := newMemFileExecutor(t)
	if err := fileExecutor.MkdirAll("/appstages", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderAll(context.Background(), fileExecutor, &bytes.Buffer{}, "/appstages", "/dest", ".")
	if err == nil {
		t.Error("expected error when no appstage-* directories found")
	}
}
