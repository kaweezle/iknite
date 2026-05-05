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

// cSpell: words dvcm mockhost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mockhost "github.com/kaweezle/iknite/mocks/pkg/host"
	"github.com/kaweezle/iknite/pkg/host"
)

type failingFileSystem struct {
	host.FileSystem
	writeFileErrs map[string]error
	mkdirAllErrs  map[string]error
}

func (f *failingFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err := f.mkdirAllErrs[path]; err != nil {
		return err
	}
	if err := f.FileSystem.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("MkdirAll %s: %w", path, err)
	}
	return nil
}

func (f *failingFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := f.writeFileErrs[path]; err != nil {
		return err
	}
	if err := f.FileSystem.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("WriteFile %s: %w", path, err)
	}
	return nil
}

const (
	configMapContent = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`
)

func TestCreateKustomizeCmd(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	out := &bytes.Buffer{}
	cmd := CreateKustomizeCmd(fs, out)

	if cmd == nil {
		t.Fatal("CreateKustomizeCmd returned nil")
	}

	if cmd.Use != "kustomize <directory> [destination]" {
		t.Errorf("expected Use to be 'kustomize <directory> [destination]', got %s", cmd.Use)
	}
}

func TestRunKustomize_MissingDirectory(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()
	var out bytes.Buffer
	err := runKustomize(fs, &out, []string{"/nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestRunKustomize_MissingKustomizationFile(t *testing.T) {
	t.Parallel()
	fs := host.NewMemMapFS()

	// Create directory but no kustomization.yaml
	err := fs.MkdirAll("/test", 0o755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	var out bytes.Buffer
	err = runKustomize(fs, &out, []string{"/test"})
	if err == nil {
		t.Error("expected error for missing kustomization.yaml, got nil")
	}
}

func TestRunKustomize_DirExistsError(t *testing.T) {
	t.Parallel()

	fs := mockhost.NewMockFileSystem(t)
	fs.EXPECT().DirExists("/test").Return(false, errors.New("stat failed"))

	err := runKustomize(fs, &bytes.Buffer{}, []string{"/test"})
	if err == nil || err.Error() != "failed to check kustomization directory: stat failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKustomize_ExistsError(t *testing.T) {
	t.Parallel()

	fs := mockhost.NewMockFileSystem(t)
	fs.EXPECT().DirExists("/test").Return(true, nil)
	fs.EXPECT().Exists("/test/kustomization.yaml").Return(false, errors.New("stat failed"))

	err := runKustomize(fs, &bytes.Buffer{}, []string{"/test"})
	if err == nil || err.Error() != "failed to check kustomization.yaml: stat failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKustomize_BuildError(t *testing.T) {
	t.Parallel()

	fs := newMemFileExecutor(t)
	writeFile(t, fs, "/test/kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- missing.yaml
`)

	err := runKustomize(fs, &bytes.Buffer{}, []string{"/test"})
	if err == nil || !strings.Contains(err.Error(), "while building kustomization in /test kustomize") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKustomize_WriteError(t *testing.T) {
	t.Parallel()

	fs := newMemFileExecutor(t)
	writeKustomizeApp(t, fs, "/test")

	err := runKustomize(fs, errWriter{err: errors.New("write failed")}, []string{"/test"})
	if err == nil || err.Error() != "failed to write kustomization output: failed to write output: write failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKustomize_SplitError(t *testing.T) {
	t.Parallel()

	baseFS := newMemFileExecutor(t)
	writeKustomizeApp(t, baseFS, "/test")
	fs := &failingFileSystem{
		FileSystem: baseFS,
		writeFileErrs: map[string]error{
			"/dest/ConfigMap-test-config.yaml": errors.New("write failed"),
		},
	}

	err := runKustomize(fs, &bytes.Buffer{}, []string{"/test", "/dest"})
	if err == nil ||
		err.Error() != "failed to split kustomization output to directory: "+
			"failed to write file /dest/ConfigMap-test-config.yaml: write failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKustomize_WithValidKustomization(t *testing.T) {
	t.Parallel()
	// This test uses the real filesystem to run a full kustomize operation
	// Skip if we're in a minimal test environment
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a simple kustomization
	kustomizationContent := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`
	// Write files
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(kustomizationContent),
		0o600,
	); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	// Test without destination (print to stdout)
	fs := host.NewDefaultHost()
	var out bytes.Buffer
	err := runKustomize(fs, &out, []string{tmpDir})
	if err != nil {
		t.Errorf("runKustomize failed: %v", err)
	}
}

func TestRunKustomize_WithDestination(t *testing.T) {
	t.Parallel()
	// This test uses the real filesystem to run a full kustomize operation
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory for input
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "output")

	// Create a simple kustomization
	kustomizationContent := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
- deployment.yaml
`

	deploymentContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

	// Write files
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(kustomizationContent),
		0o600,
	); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(deploymentContent), 0o600); err != nil {
		t.Fatalf("failed to write deployment.yaml: %v", err)
	}

	// Test with destination
	fs := host.NewDefaultHost()
	out := &bytes.Buffer{}
	err := runKustomize(fs, out, []string{tmpDir, destDir})
	if err != nil {
		t.Fatalf("runKustomize failed: %v", err)
	}

	// Check that files were created
	files, err := fs.ReadDir(destDir)
	if err != nil {
		t.Fatalf("failed to read destination directory: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files in destination, got %d", len(files))
	}

	// Check that the files have the correct naming pattern (CamelCase kind with underscore for colons)
	expectedFiles := map[string]bool{
		"ConfigMap-test-config.yaml":      false,
		"Deployment-test-deployment.yaml": false,
	}

	for _, file := range files {
		if _, ok := expectedFiles[file.Name()]; ok {
			expectedFiles[file.Name()] = true
		}
	}

	for filename, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %s not found in destination", filename)
		}
	}
}

func TestKustomizeCmd_Integration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory for input
	tmpDir := t.TempDir()

	// Create a simple kustomization
	kustomizationContent := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`

	// Write files
	if err := os.WriteFile(
		filepath.Join(tmpDir, "kustomization.yaml"),
		[]byte(kustomizationContent),
		0o600,
	); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o600); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	// Create command and execute
	fs := host.NewDefaultHost()
	// Capture output
	var stdout bytes.Buffer
	cmd := CreateKustomizeCmd(fs, &stdout)

	// Set args
	cmd.SetArgs([]string{tmpDir})

	// Execute
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	// Check output contains expected content
	output := stdout.String()
	if !strings.Contains(output, "ConfigMap") || !strings.Contains(output, "test-config") {
		t.Errorf("expected output to contain ConfigMap and test-config, got: %s", output)
	}
}
