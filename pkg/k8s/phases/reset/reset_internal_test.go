/*
Copyright 2019 The Kubernetes Authors.

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
package reset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

func TestResetPhaseConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		constructor func() workflow.Phase
		name        string
		wantName    string
	}{
		{name: "cleanup-config", constructor: NewCleanupConfigPhase, wantName: "cleanup-config"},
		{name: "cleanup-node", constructor: NewCleanupNodePhase, wantName: "cleanup-node"},
		{name: "cleanup-service", constructor: NewCleanupServicePhase, wantName: "cleanup-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			phase := tt.constructor()
			req.Equal(tt.wantName, phase.Name)
			req.NotNil(phase.Run)
			req.NotEmpty(phase.Short)
		})
	}
}

func TestRunPhasesRejectInvalidData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		run  func(workflow.RunData) error
		name string
	}{
		{name: "cleanup-config", run: runCleanupConfig},
		{name: "cleanup-node", run: runCleanupNode},
		{name: "cleanup-service", run: runCleanupService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			err := tt.run(struct{}{})
			req.Error(err)
			req.Contains(err.Error(), "invalid data struct")
		})
	}
}

func TestCleanDirAndIsDirEmpty(t *testing.T) {
	t.Parallel()

	t.Run("clean dir and keep directory", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		dir := t.TempDir()
		req.NoError(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600))
		req.NoError(os.MkdirAll(filepath.Join(dir, "nested"), 0o750))
		req.NoError(os.WriteFile(filepath.Join(dir, "nested", "b.txt"), []byte("b"), 0o600))

		req.NoError(CleanDir(dir))
		empty, err := IsDirEmpty(dir)
		req.NoError(err)
		req.True(empty)
	})

	t.Run("missing directory returns nil", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		err := CleanDir(filepath.Join(t.TempDir(), "missing"))
		req.NoError(err)
	})

	t.Run("is dir empty on non-empty directory", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		dir := t.TempDir()
		req.NoError(os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0o600))

		empty, err := IsDirEmpty(dir)
		req.NoError(err)
		req.False(empty)
	})

	t.Run("is dir empty returns error for missing path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		_, err := IsDirEmpty(filepath.Join(t.TempDir(), "missing"))
		req.Error(err)
	})
}

func TestResetConfigDirDeletesExpectedFiles(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	configDir := t.TempDir()
	manifestDir := filepath.Join(configDir, "manifests")
	req.NoError(os.MkdirAll(manifestDir, 0o750))
	req.NoError(os.WriteFile(filepath.Join(manifestDir, "kube-apiserver.yaml"), []byte("x"), 0o600))

	filesToCreate := []string{
		kubeadmConstants.AdminKubeConfigFileName,
		kubeadmConstants.SuperAdminKubeConfigFileName,
		kubeadmConstants.KubeletKubeConfigFileName,
		kubeadmConstants.KubeletBootstrapKubeConfigFileName,
		kubeadmConstants.ControllerManagerKubeConfigFileName,
		kubeadmConstants.SchedulerKubeConfigFileName,
	}

	for _, fileName := range filesToCreate {
		req.NoError(os.WriteFile(filepath.Join(configDir, fileName), []byte("conf"), 0o600))
	}

	resetConfigDir(configDir, []string{manifestDir}, false)

	empty, err := IsDirEmpty(manifestDir)
	req.NoError(err)
	req.True(empty)

	for _, fileName := range filesToCreate {
		_, statErr := os.Stat(filepath.Join(configDir, fileName))
		req.Error(statErr)
		req.ErrorIs(statErr, os.ErrNotExist)
	}
}
