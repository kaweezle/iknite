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

	"github.com/kaweezle/iknite/pkg/host"
)

const testDir = "test"

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

		fs := host.NewMemMapFS()
		req.NoError(fs.MkdirAll(testDir, os.FileMode(0o755)))
		req.NoError(fs.WriteFile(filepath.Join(testDir, "a.txt"), []byte("a"), 0o600))
		req.NoError(fs.MkdirAll(filepath.Join(testDir, "nested"), 0o750))
		req.NoError(fs.WriteFile(filepath.Join(testDir, "nested", "b.txt"), []byte("b"), 0o600))

		req.NoError(CleanDir(fs, testDir))
		empty, err := IsDirEmpty(fs, testDir)
		req.NoError(err)
		req.True(empty)
	})

	t.Run("missing directory returns nil", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := host.NewMemMapFS()
		err := CleanDir(fs, "missing")
		req.NoError(err)
	})

	t.Run("is dir empty on non-empty directory", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := host.NewMemMapFS()
		req.NoError(fs.MkdirAll(testDir, os.FileMode(0o755)))
		req.NoError(fs.WriteFile(filepath.Join(testDir, "x"), []byte("x"), 0o600))

		empty, err := IsDirEmpty(fs, testDir)
		req.NoError(err)
		req.False(empty)
	})

	t.Run("is dir empty returns error for missing path", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)
		fs := host.NewMemMapFS()
		_, err := IsDirEmpty(fs, "missing")
		req.Error(err)
	})
}

func TestResetConfigDirDeletesExpectedFiles(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	fs := host.NewMemMapFS()
	req.NoError(fs.MkdirAll(testDir, os.FileMode(0o755)))
	manifestDir := filepath.Join(testDir, "manifests")
	req.NoError(fs.MkdirAll(manifestDir, 0o750))
	req.NoError(fs.WriteFile(filepath.Join(manifestDir, "kube-apiserver.yaml"), []byte("x"), 0o600))

	filesToCreate := []string{
		kubeadmConstants.AdminKubeConfigFileName,
		kubeadmConstants.SuperAdminKubeConfigFileName,
		kubeadmConstants.KubeletKubeConfigFileName,
		kubeadmConstants.KubeletBootstrapKubeConfigFileName,
		kubeadmConstants.ControllerManagerKubeConfigFileName,
		kubeadmConstants.SchedulerKubeConfigFileName,
	}

	for _, fileName := range filesToCreate {
		req.NoError(fs.WriteFile(filepath.Join(testDir, fileName), []byte("conf"), 0o600))
	}

	resetConfigDir(fs, testDir, []string{manifestDir}, false)

	empty, err := IsDirEmpty(fs, manifestDir)
	req.NoError(err)
	req.True(empty)

	for _, fileName := range filesToCreate {
		_, statErr := fs.Stat(filepath.Join(testDir, fileName))
		req.Error(statErr)
		req.ErrorIs(statErr, os.ErrNotExist)
	}
}
