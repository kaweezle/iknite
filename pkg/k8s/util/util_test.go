package util_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s/util"
)

func TestReadStaticPodFromDisk(t *testing.T) {
	t.Parallel()

	const (
		manifestPath = "/manifests/static-pod.yaml"
		podManifest  = `apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
    - name: kube-apiserver
      image: registry.k8s.io/kube-apiserver:v1.32.0
`
		nonPodManifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-pod
  namespace: kube-system
`
		invalidManifest = "not: [valid"
	)

	testCases := []struct {
		name            string
		manifestContent string
		wantErrContains string
		wantName        string
		createManifest  bool
	}{
		{
			name:            "returns error when manifest file does not exist",
			createManifest:  false,
			wantErrContains: "failed to read manifest",
		},
		{
			name:            "returns error when manifest yaml is invalid",
			manifestContent: invalidManifest,
			createManifest:  true,
			wantErrContains: "failed to unmarshal manifest",
		},
		{
			name:            "returns error when manifest is not a pod",
			manifestContent: nonPodManifest,
			createManifest:  true,
			wantErrContains: "failed to parse Pod object",
		},
		{
			name:            "returns parsed pod",
			manifestContent: podManifest,
			createManifest:  true,
			wantName:        "kube-apiserver",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := host.NewMemMapFS()
			req := require.New(t)

			if tc.createManifest {
				err := fs.MkdirAll("/manifests", 0o755)
				req.NoError(err)

				err = fs.WriteFile(manifestPath, []byte(tc.manifestContent), os.FileMode(0o644))
				req.NoError(err)
			}

			pod, err := util.ReadStaticPodFromDisk(fs, manifestPath)

			if tc.wantErrContains != "" {
				req.Error(err)
				req.ErrorContains(err, tc.wantErrContains)
				req.NotNil(pod)
				req.Empty(pod.Name)
				return
			}

			req.NoError(err)
			req.NotNil(pod)
			req.Equal(tc.wantName, pod.Name)
		})
	}
}
