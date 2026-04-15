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
package init

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"
)

func TestPhaseConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		constructor func() workflow.Phase
		name        string
		wantName    string
	}{
		{name: "pre-clean", constructor: NewPreCleanHostPhase, wantName: "pre-clean-host"},
		{name: "prepare", constructor: NewPrepareHostPhase, wantName: "prepare-host"},
		{name: "kubelet", constructor: NewKubeletStartPhase, wantName: "kubelet-start"},
		{name: "kine", constructor: NewKineControlPlanePhase, wantName: "kine"},
		{name: "kube-vip", constructor: NewKubeVipControlPlanePhase, wantName: "kube-vip"},
		{name: "kustomize", constructor: NewKustomizeClusterPhase, wantName: "kustomize-cluster"},
		{name: "mdns", constructor: NewMDnsPublishPhase, wantName: "mdns-publish"},
		{name: "serve", constructor: NewServePhase, wantName: "serve"},
		{name: "copy-config", constructor: NewCopyConfigPhase, wantName: "copy-config"},
		{name: "workloads", constructor: NewWorkloadsPhase, wantName: "workloads"},
		{name: "daemonize", constructor: NewDaemonizePhase, wantName: "daemonize"},
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
		{name: "pre-clean", run: runPreCleanHost},
		{name: "prepare", run: runPrepareHost},
		{name: "kubelet", run: runKubeletStart},
		{name: "kine", run: runKineControlPlane},
		{name: "kube-vip", run: runKubeVipControlPlane},
		{name: "kustomize", run: runKustomize},
		{name: "mdns", run: runMDnsPublish},
		{name: "serve", run: runServe},
		{name: "copy-config", run: runCopyConfig},
		{name: "workloads", run: runMonitorWorkloads},
		{name: "daemonize", run: runDaemonize},
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

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prepare func(t *testing.T) (string, string)
		name    string
		wantErr bool
	}{
		{
			name: "copies file and creates destination directory",
			prepare: func(t *testing.T) (string, string) {
				t.Helper()
				dir := t.TempDir()
				src := filepath.Join(dir, "src", "config.conf")
				dst := filepath.Join(dir, "dst", "nested", "config.conf")
				req := require.New(t)
				req.NoError(os.MkdirAll(filepath.Dir(src), 0o750))
				req.NoError(os.WriteFile(src, []byte("payload"), 0o600))
				return src, dst
			},
			wantErr: false,
		},
		{
			name: "missing source returns error",
			prepare: func(t *testing.T) (string, string) {
				t.Helper()
				dir := t.TempDir()
				return filepath.Join(dir, "missing"), filepath.Join(dir, "dest")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			src, dst := tt.prepare(t)
			err := copyFile(src, dst)
			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			content, readErr := os.ReadFile(dst) //nolint:gosec // test temp path
			req.NoError(readErr)
			req.Equal("payload", string(content))
		})
	}
}

func TestWaitForKubelet(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	req.NoError(cmd.Start())

	canceled := false
	err := WaitForKubelet(cmd, nil, func() { canceled = true })
	req.Error(err)
	req.Contains(err.Error(), "failed to wait for kubelet")
	req.True(canceled)
}
