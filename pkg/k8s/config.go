package k8s

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/antoinemartin/k8wsl/pkg/constants"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Config api.Config

// LoadFromDefault loads the configuration from the default admin.conf file,
// usually located at /etc/kubernetes/admin.conf.
func LoadFromDefault() (*Config, error) {
	_config, err := clientcmd.LoadFromFile(constants.KubernetesAdminConfig)
	if err != nil {
		return nil, err
	}
	config := (*Config)(_config)
	return config, nil
}

// RenameConfig changes the name of the cluster and the context from the
// default (kubernetes) to newName in c.
func (c *Config) RenameConfig(newName string) *Config {
	newClusters := make(map[string]*api.Cluster)
	for _, v := range c.Clusters {
		newClusters[newName] = v
	}
	c.Clusters = newClusters

	newContexts := make(map[string]*api.Context)
	for _, v := range c.Contexts {
		newContexts[newName] = v
		v.Cluster = newName
	}
	c.Contexts = newContexts

	c.CurrentContext = newName
	return c
}

// IsConfigServerAddress checks that config points to the server at ip IP
// address
func (config *Config) IsConfigServerAddress(ip net.IP) bool {
	expectedURL := fmt.Sprintf("https://%v:6443", ip)
	for _, cluster := range config.Clusters {
		if cluster.Server != expectedURL {
			return false
		}
	}
	return true
}

// CheckClusterRunning checks that the cluster is running by requesting the
// API server /readyz endpoint. It checks 10 times and waits for 2 seconds
// between each check.
func (config *Config) CheckClusterRunning() error {

	clientconfig := clientcmd.NewDefaultClientConfig(api.Config(*config), nil)
	rest, err := clientconfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(rest)
	if err != nil {
		return err
	}

	retries := 10
	oktries := 0
	query := client.Discovery().RESTClient().Get().AbsPath("/readyz")
	for retries > 0 {
		content, err := query.DoRaw(context.Background())
		if err == nil {
			contentStr := string(content)
			if contentStr != "ok" {
				err = fmt.Errorf("cluster health API returned: %s", contentStr)
			} else {
				oktries = oktries + 1
				log.WithField("oktries", oktries).Trace("Ok ressponse from server")
				if oktries == 2 {
					break
				}
			}
		}

		retries = retries - 1
		if retries == 0 {
			log.Trace("No more retries left.")
			return err
		} else {
			log.WithField("err", err).Debug("Waiting 2 seconds...")
			time.Sleep(2 * time.Second)
		}
	}

	return err
}

// WriteToFile writes the config configuration to the file pointed by filename.
// it returns the appropriate error in case of failure.
func (config *Config) WriteToFile(filename string) error {
	return clientcmd.WriteToFile(*(*api.Config)(config), filename)
}
