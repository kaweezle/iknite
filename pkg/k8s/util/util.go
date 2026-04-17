// cSpell: words kubeadmutil
package util

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	"github.com/kaweezle/iknite/pkg/host"
)

func ReadStaticPodFromDisk(fs host.FileSystem, manifestPath string) (*v1.Pod, error) {
	buf, err := fs.ReadFile(manifestPath)
	if err != nil {
		return &v1.Pod{}, fmt.Errorf("failed to read manifest for %q: %w", manifestPath, err)
	}

	obj, err := kubeadmutil.UniversalUnmarshal(buf)
	if err != nil {
		return &v1.Pod{}, fmt.Errorf("failed to unmarshal manifest for %q: %w", manifestPath, err)
	}

	pod, ok := obj.(*v1.Pod)
	if !ok {
		return &v1.Pod{}, fmt.Errorf("failed to parse Pod object defined in %q", manifestPath)
	}

	return pod, nil
}
