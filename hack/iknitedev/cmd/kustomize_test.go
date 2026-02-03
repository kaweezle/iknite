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

// cSpell: words dvcm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestCreateKustomizeCmd(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	cmd := CreateKustomizeCmd(fs)

	if cmd == nil {
		t.Fatal("CreateKustomizeCmd returned nil")
	}

	if cmd.Use != "kustomize <directory> [destination]" {
		t.Errorf("expected Use to be 'kustomize <directory> [destination]', got %s", cmd.Use)
	}
}

func TestRunKustomize_MissingDirectory(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()

	err := runKustomize(fs, []string{"/nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestRunKustomize_MissingKustomizationFile(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()

	// Create directory but no kustomization.yaml
	err := fs.MkdirAll("/test", 0o755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	err = runKustomize(fs, []string{"/test"})
	if err == nil {
		t.Error("expected error for missing kustomization.yaml, got nil")
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

	configMapContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`

	// Write files
	if err := os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"), []byte(kustomizationContent), 0o644); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o644); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	// Test without destination (print to stdout)
	fs := afero.NewOsFs()
	err := runKustomize(fs, []string{tmpDir})
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

	configMapContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
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
	if err := os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"), []byte(kustomizationContent), 0o644); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o644); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(deploymentContent), 0o644); err != nil {
		t.Fatalf("failed to write deployment.yaml: %v", err)
	}

	// Test with destination
	fs := afero.NewOsFs()
	err := runKustomize(fs, []string{tmpDir, destDir})
	if err != nil {
		t.Fatalf("runKustomize failed: %v", err)
	}

	// Check that files were created
	files, err := afero.ReadDir(fs, destDir)
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

func TestSplitResources(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	destDir := "/output"

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

	err := splitResources(fs, yamlData, destDir)
	if err != nil {
		t.Fatalf("splitResources failed: %v", err)
	}

	// Check that files were created
	files, err := afero.ReadDir(fs, destDir)
	if err != nil {
		t.Fatalf("failed to read destination directory: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Check file contents (CamelCase kind with underscore for colons)
	configMapPath := filepath.Join(destDir, "ConfigMap-test-config.yaml")
	exists, err := afero.Exists(fs, configMapPath)
	if err != nil {
		t.Fatalf("failed to check if ConfigMap file exists: %v", err)
	}
	if !exists {
		t.Error("ConfigMap file was not created")
	}

	secretPath := filepath.Join(destDir, "Secret-test-secret.yaml")
	exists, err = afero.Exists(fs, secretPath)
	if err != nil {
		t.Fatalf("failed to check if secret file exists: %v", err)
	}
	if !exists {
		t.Error("secret file was not created")
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

	configMapContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`

	// Write files
	if err := os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"), []byte(kustomizationContent), 0o644); err != nil {
		t.Fatalf("failed to write kustomization.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "configmap.yaml"), []byte(configMapContent), 0o644); err != nil {
		t.Fatalf("failed to write configmap.yaml: %v", err)
	}

	// Create command and execute
	fs := afero.NewOsFs()
	cmd := CreateKustomizeCmd(fs)

	// Capture output
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

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
