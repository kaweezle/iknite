// cSpell: words paralleltest testpackage apimachinery metav1 clientcmd
//
//nolint:paralleltest // tests operate on package/global config and fake client objects
package k8s

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestConfigRenameAndAddressHelpers(t *testing.T) {
	req := require.New(t)

	config := &Config{
		Clusters:       map[string]*api.Cluster{"kubernetes": {Server: "https://10.0.0.2:6443"}},
		Contexts:       map[string]*api.Context{"kubernetes": {Cluster: "kubernetes", AuthInfo: "kubernetes"}},
		AuthInfos:      map[string]*api.AuthInfo{"kubernetes": {}},
		CurrentContext: "kubernetes",
	}

	config.RenameConfig("iknite")
	req.Contains(config.Clusters, "iknite")
	req.Contains(config.Contexts, "iknite")
	req.Contains(config.AuthInfos, "iknite")
	req.Equal("iknite", config.CurrentContext)
	req.True(config.IsConfigServerAddress("10.0.0.2"))
	req.False(config.IsConfigServerAddress("10.0.0.9"))
}

func TestConfigWriteToFileAndLoadFromFile(t *testing.T) {
	req := require.New(t)

	config := &Config{
		Clusters:       map[string]*api.Cluster{"iknite": {Server: "https://127.0.0.1:6443"}},
		Contexts:       map[string]*api.Context{"iknite": {Cluster: "iknite", AuthInfo: "iknite"}},
		AuthInfos:      map[string]*api.AuthInfo{"iknite": {}},
		CurrentContext: "iknite",
	}

	path := filepath.Join(t.TempDir(), "config")
	req.NoError(config.WriteToFile(path))

	loaded, err := LoadFromFile(path)
	req.NoError(err)
	req.Equal("iknite", loaded.CurrentContext)
	req.True(loaded.IsConfigServerAddress("127.0.0.1"))
}

func TestGetAndWriteIkniteConfigMap(t *testing.T) {
	req := require.New(t)

	client := fake.NewSimpleClientset()
	ctx := context.Background()

	cm, err := GetIkniteConfigMap(ctx, client)
	req.NoError(err)
	req.Equal("iknite-config", cm.Name)
	req.Equal("false", cm.Data["configured"])

	written, err := WriteIkniteConfigMap(ctx, client, cm)
	req.NoError(err)
	req.Equal("iknite-config", written.Name)
	req.Equal("kube-system", written.Namespace)

	written.Data["configured"] = "true"
	written.UID = "uid-1"
	updated, err := WriteIkniteConfigMap(ctx, client, written)
	req.NoError(err)
	req.Equal("true", updated.Data["configured"])
}

func TestGetIkniteConfigMapExisting(t *testing.T) {
	req := require.New(t)

	client := fake.NewSimpleClientset(&coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "iknite-config", Namespace: "kube-system"},
		Data:       map[string]string{"configured": "true"},
	})

	cm, err := GetIkniteConfigMap(context.Background(), client)
	req.NoError(err)
	req.Equal("true", cm.Data["configured"])
}
