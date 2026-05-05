package k8s

import (
	"context"
	"fmt"
	"time"

	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kaweezle/iknite/pkg/alpine"
	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/utils"
)

// defaultKubeletRuntime is the production implementation of KubeletRuntime.
// It holds the host and kubeClient so callers of StartAndConfigureKubelet do
// not need to pass them explicitly.
type defaultKubeletRuntime struct {
	fileExec   host.FileExecutor
	kubeClient resource.RESTClientGetter
}

var _ KubeletRuntime = (*defaultKubeletRuntime)(nil)

// NewKubeletRuntime returns a KubeletRuntime backed by the real k8s functions.
func NewKubeletRuntime(fileExec host.FileExecutor, kubeClient resource.RESTClientGetter) KubeletRuntime { // nocov
	return &defaultKubeletRuntime{fileExec: fileExec, kubeClient: kubeClient}
}

func (r *defaultKubeletRuntime) StartKubelet(ctx context.Context) (host.Process, error) { // nocov
	return StartKubelet(ctx, r.fileExec)
}

func (r *defaultKubeletRuntime) CheckKubeletRunning(
	ctx context.Context,
	retries, okResponses int,
	waitTime time.Duration,
) error { // nocov
	return CheckKubeletRunning(ctx, retries, okResponses, waitTime)
}

func (r *defaultKubeletRuntime) CheckClusterRunning(
	ctx context.Context,
	retries, okResponses int,
	interval time.Duration,
) error { // nocov
	client, err := RESTClient(r.kubeClient)
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}
	return CheckClusterRunning(ctx, client, retries, okResponses, interval)
}

func (r *defaultKubeletRuntime) Kustomize(
	ctx context.Context,
	options *utils.KustomizeOptions,
) error { // nocov
	return Kustomize(ctx, r.kubeClient, r.fileExec, options)
}

func (r *defaultKubeletRuntime) RemovePidFile() { // nocov
	alpine.RemovePidFile(r.fileExec, KubeletName)
}
