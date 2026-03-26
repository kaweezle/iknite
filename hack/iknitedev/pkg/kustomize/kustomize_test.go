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
package kustomize_test

// cSpell: words kustomizer dvcm

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/kaweezle/iknite/hack/iknitedev/pkg/kustomize"
)

const configMapContent = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`

func TestSplitYAMLToDir(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/output", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	yamlData := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  password: cGFzc3dvcmQ=
`)

	if err := kustomize.SplitYAMLToDir(fs, yamlData, "/output"); err != nil {
		t.Fatalf("SplitYAMLToDir failed: %v", err)
	}

	files, err := afero.ReadDir(fs, "/output")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	for _, name := range []string{"ConfigMap-test-config.yaml", "Secret-test-secret.yaml"} {
		ok, err := afero.Exists(fs, filepath.Join("/output", name)) //nolint:gocritic // Memory fs
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if !ok {
			t.Errorf("expected file %s not found", name)
		}
	}
}

func TestSplitYAMLToDir_MissingKind(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	yamlData := []byte(`apiVersion: v1
metadata:
  name: no-kind
`)
	err := kustomize.SplitYAMLToDir(fs, yamlData, "/output")
	if err == nil {
		t.Error("expected error for resource missing 'kind' field")
	}
}

func TestSplitYAMLToDir_MissingMetadata(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	yamlData := []byte(`apiVersion: v1
kind: ConfigMap
`)
	err := kustomize.SplitYAMLToDir(fs, yamlData, "/output")
	if err == nil {
		t.Error("expected error for resource missing 'metadata' field")
	}
}

func TestSplitYAMLToDir_CreatesDestDir(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	// directory does not exist yet
	yamlData := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
`)
	if err := kustomize.SplitYAMLToDir(fs, yamlData, "/new/dir"); err != nil {
		t.Fatalf("SplitYAMLToDir: %v", err)
	}
	ok, err := afero.Exists(fs, "/new/dir/ConfigMap-cm.yaml")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("expected ConfigMap-cm.yaml to be created")
	}
}

func TestBuild_Integration(t *testing.T) {
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

	resources, err := kustomize.Build(tmpDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if resources == nil {
		t.Fatal("Build returned nil resources")
	}
	if len(resources.Resources()) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources.Resources()))
	}
}

func TestWriteToWriter_Integration(t *testing.T) {
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

	resources, err := kustomize.Build(tmpDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf bytes.Buffer
	if err := kustomize.WriteToWriter(resources, &buf); err != nil {
		t.Fatalf("WriteToWriter: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("ConfigMap")) {
		t.Errorf("expected ConfigMap in output, got: %s", buf.String())
	}
}

func TestSplitResMapToDir_Integration(t *testing.T) {
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

	resources, err := kustomize.Build(tmpDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	fs := afero.NewOsFs()
	if err := kustomize.SplitResMapToDir(fs, resources, destDir); err != nil { //nolint:govet // integration test
		t.Fatalf("SplitResMapToDir: %v", err)
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
