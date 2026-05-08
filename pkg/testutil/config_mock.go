package testutil

import (
	"fmt"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/kaweezle/iknite/pkg/host"
)

func GetBasicConfigContent(url string) ([]byte, error) {
	if url == "" {
		url = "https://127.0.0.1:6443"
	}
	config := &api.Config{
		Clusters:       map[string]*api.Cluster{"iknite": {Server: url}},
		Contexts:       map[string]*api.Context{"iknite": {Cluster: "iknite", AuthInfo: "iknite"}},
		AuthInfos:      map[string]*api.AuthInfo{"iknite": {}},
		CurrentContext: "iknite",
	}

	content, err := clientcmd.Write(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize kubeconfig: %w", err)
	}
	return content, nil
}

func CreateBasicConfig(fs host.FileSystem, path, url string) error {
	content, err := GetBasicConfigContent(url)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory for kubeconfig file: %w", err)
	}
	if err := fs.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %w", err)
	}
	return nil
}
