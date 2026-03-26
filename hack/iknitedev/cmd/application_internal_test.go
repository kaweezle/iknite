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
package cmd

// cSpell: words dvcm appstage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

// ---- helpers ----------------------------------------------------------------

func writeFile(t *testing.T, fs afero.Fs, path, content string) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// ---- detectAppType ----------------------------------------------------------

func TestDetectAppType_Kustomize(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	writeFile(t, fs, "/app/kustomization.yaml", "")

	typ, path, err := detectAppType(fs, "/app")
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
	fs := afero.NewMemMapFs()
	writeFile(t, fs, "/app/helmfile.yaml", "")

	typ, path, err := detectAppType(fs, "/app")
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
	fs := afero.NewMemMapFs()
	writeFile(t, fs, "/app/helmfile.yaml.gotmpl", "")

	typ, path, err := detectAppType(fs, "/app")
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
	fs := afero.NewMemMapFs()
	writeFile(t, fs, "/app/Chart.yaml", "")

	typ, path, err := detectAppType(fs, "/app")
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
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/app", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	typ, _, err := detectAppType(fs, "/app")
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
	fs := afero.NewMemMapFs()
	out := &bytes.Buffer{}
	cmd := CreateApplicationCmd(fs, out)
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
	fs := afero.NewMemMapFs()
	err := runValidateApp(fs, &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_MissingDir(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	err := runRenderApp(fs, &bytes.Buffer{}, "/nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunRenderApp_UnknownType(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/app", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderApp(fs, &bytes.Buffer{}, "/app", "")
	if err == nil {
		t.Error("expected error for directory with no recognized app definition")
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

	fs := afero.NewOsFs()
	var out bytes.Buffer
	if err := runRenderApp(fs, &out, tmpDir, ""); err != nil {
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

	fs := afero.NewOsFs()
	if err := runRenderApp(fs, &bytes.Buffer{}, tmpDir, destDir); err != nil {
		t.Fatalf("runRenderApp: %v", err)
	}

	files, err := afero.ReadDir(fs, destDir)
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
	fs := afero.NewMemMapFs()

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
	writeFile(t, fs, "/manifests/Application-my-app.yaml", appYAML)
	writeFile(t, fs, "/manifests/ConfigMap-cfg.yaml", nonAppYAML)

	apps, err := parseApplicationsFromDir(fs, "/manifests")
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
	fs := afero.NewMemMapFs()
	err := runRenderAll(fs, &bytes.Buffer{}, "/nonexistent", "/dest", ".")
	if err == nil {
		t.Error("expected error for nonexistent appstages directory")
	}
}

func TestRunRenderAll_NoAppstages(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/appstages", 0o755); err != nil {
		t.Fatal(err)
	}
	err := runRenderAll(fs, &bytes.Buffer{}, "/appstages", "/dest", ".")
	if err == nil {
		t.Error("expected error when no appstage-* directories found")
	}
}
