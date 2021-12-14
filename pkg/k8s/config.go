package k8s

import "k8s.io/client-go/tools/clientcmd/api"

// RenameConfig changes the name of the cluster and the context from the
// default (kubernetes) to newName in c.
func RenameConfig(c *api.Config, newName string) *api.Config {
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
