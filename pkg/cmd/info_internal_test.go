// cSpell: words  stretchr paralleltest
//
//nolint:gosec // in-package command tests require global stdout/viper and temp file path reads
package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/apis/iknite/v1alpha1"
	"github.com/kaweezle/iknite/pkg/cmd/options"
)

//nolint:paralleltest // Changes stdout
func TestNewInfoCmdAndVersionsCmd(t *testing.T) {
	req := require.New(t)
	spec := &v1alpha1.IkniteClusterSpec{Ip: []byte{127, 0, 0, 1}}

	infoCmd := NewInfoCmd(spec)
	req.NotNil(infoCmd)
	req.Equal("info", infoCmd.Name())

	imagesCmd, _, err := infoCmd.Find([]string{"images"})
	req.NoError(err)
	req.Equal("images", imagesCmd.Name())

	versionsCmd := NewVersionsCmd()
	req.NotNil(versionsCmd)
	req.Equal("versions", versionsCmd.Name())

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	req.NoError(pipeErr)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	versionsCmd.Run(versionsCmd, nil)
	req.NoError(w.Close())
	out, err := io.ReadAll(r)
	req.NoError(err)
	req.Contains(string(out), "Iknite Version")
	req.Contains(string(out), "Default Kubernetes Version")
}

//nolint:paralleltest // mutates viper
func TestPerformInfoWritesFile(t *testing.T) {
	req := require.New(t)
	viper.Reset()
	t.Cleanup(viper.Reset)

	destination := filepath.Join(t.TempDir(), "config.json")
	viper.Set(options.OutputFormat, "json")
	viper.Set(options.OutputDestination, destination)

	performInfo(&v1alpha1.IkniteClusterSpec{ClusterName: "demo", Ip: []byte{127, 0, 0, 1}})

	content, err := os.ReadFile(destination)
	req.NoError(err)
	req.Contains(string(content), "clusterName")
	req.Contains(string(content), "demo")
}
