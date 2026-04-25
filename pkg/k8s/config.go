/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

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
package k8s

// cSpell: words clientcmd readyz polymorphichelpers objectrestarter
// cSpell: disable
import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	k8Errors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	kubeadmConstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/provision"
	"github.com/kaweezle/iknite/pkg/utils"
)

// cSpell: enable

const configuredValueTrue = "true"

// LoadFromFile loads the configuration from the file specified by filename.
func LoadFromFile(fs host.FileSystem, filename string) (*api.Config, error) {
	content, err := fs.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	config, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest kubeconfig file %s content: %w", filename, err)
	}

	// set LocationOfOrigin on every Cluster, User, and Context
	for key, obj := range config.AuthInfos {
		obj.LocationOfOrigin = filename
		config.AuthInfos[key] = obj
	}
	for key, obj := range config.Clusters {
		obj.LocationOfOrigin = filename
		config.Clusters[key] = obj
	}
	for key, obj := range config.Contexts {
		obj.LocationOfOrigin = filename
		config.Contexts[key] = obj
	}

	if config.AuthInfos == nil {
		config.AuthInfos = map[string]*api.AuthInfo{}
	}
	if config.Clusters == nil {
		config.Clusters = map[string]*api.Cluster{}
	}
	if config.Contexts == nil {
		config.Contexts = map[string]*api.Context{}
	}

	return config, nil
}

// LoadFromDefault loads the configuration from the default admin.conf file,
// usually located at /etc/kubernetes/admin.conf.
func LoadFromDefault(fs host.FileSystem) (*api.Config, error) {
	return LoadFromFile(fs, kubeadmConstants.GetAdminKubeConfigPath())
}

// ClientSetFromFile returns a Kubernetes clientset for the configuration specified.
func ClientSetFromFile(fs host.FileSystem, path string) (kubernetes.Interface, error) {
	client, err := NewClientFromFile(fs, path)
	if err != nil {
		return nil, err
	}
	return ClientSet(client)
}

// RenameConfig changes the name of the cluster and the context from the
// default (kubernetes) to newName in c.
func RenameConfig(c *api.Config, newName string) *api.Config {
	newUsers := make(map[string]*api.AuthInfo)
	for _, v := range c.AuthInfos {
		newUsers[newName] = v
	}
	c.AuthInfos = newUsers

	newClusters := make(map[string]*api.Cluster)
	for _, v := range c.Clusters {
		newClusters[newName] = v
	}
	c.Clusters = newClusters

	newContexts := make(map[string]*api.Context)
	for _, v := range c.Contexts {
		newContexts[newName] = v
		v.Cluster = newName
		v.AuthInfo = newName
	}
	c.Contexts = newContexts

	c.CurrentContext = newName
	return c
}

// IsConfigServerAddress checks that config points to the server at ip IP
// address.
func IsConfigServerAddress(c resource.RESTClientGetter, address string) bool {
	expectedURL := fmt.Sprintf("https://%v:6443", address)

	restConfig, err := c.ToRESTConfig()
	if err != nil {
		return false
	}
	return restConfig.Host == expectedURL
}

// CheckClusterRunning checks that the cluster is running by requesting the
// API server /readyz endpoint. It checks retries times and waits for waitTime
// milliseconds between each check. It needs at least okResponses good responses
// from the server.
func CheckClusterRunning(
	ctx context.Context,
	client rest.Interface,
	retries, okResponses int,
	interval time.Duration,
) error {
	var err error
	okTries := 0
	query := client.Get().AbsPath("/readyz")
	first := true
	for ; retries > 0; retries-- {
		if !first {
			log.WithFields(log.Fields{
				"err":       err,
				"wait_time": interval,
			}).Debug("Waiting...")
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled: %w", ctx.Err())
			case <-time.After(interval):
			}
		}
		first = false

		var content []byte
		content, err = query.DoRaw(ctx)
		if err != nil {
			log.WithError(err).Debug("while querying cluster readiness")
			continue
		}

		contentStr := string(content)
		if contentStr != "ok" {
			err = fmt.Errorf("cluster health API returned: %s", contentStr)
			log.WithError(err).Debug("Bad response")
		} else {
			okTries++
			log.WithField("okTries", okTries).Trace("Ok response from server")
			if okTries == okResponses {
				break
			}
		}
	}

	if retries == 0 && okTries < okResponses {
		log.Trace("No more retries left.")
	}

	return err
}

// WriteToFile writes the config configuration to the file pointed by filename.
// It returns the appropriate error in case of failure.
func WriteToFile(config *api.Config, fs host.FileSystem, filename string) error {
	content, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failed to serialize kubeconfig: %w", err)
	}
	if err := fs.WriteFile(filename, content, 0o644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %w", err)
	}
	return nil
}

// RestartProxy restarts kube-proxy after config has been updated. This needs to
// be done after an IP address change.
// The restart method is taken from kubectl:
// https://github.com/kubernetes/kubectl/blob/
// 652881798563c00c1895ded6ced819030bfaa4d7/pkg/polymorphichelpers/objectrestarter.go#L81.
func RestartProxy(ctx context.Context, client kubernetes.Interface) error {
	dsi := client.AppsV1().DaemonSets("kube-system")
	var err error
	var ds *appsV1.DaemonSet

	if ds, err = dsi.Get(ctx, "kube-proxy", metaV1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to get kube-proxy daemonset: %w", err)
	}

	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().
		Format(time.RFC3339)

	_, err = dsi.Update(ctx, ds, metaV1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update kube-proxy daemonset: %w", err)
	}
	return nil
}

// Kustomize applies Kubernetes kustomizations to configure the cluster.
// It takes an outbound IP address, a kustomization path or content, a force flag,
// and a wait timeout in seconds.
//
// The function checks if configuration has already been applied by reading the
// 'configured' field in the iknite ConfigMap. If already configured and force is
// false, it logs a warning and skips configuration. Otherwise, it applies the
// provided kustomization and  marks the cluster as configured by updating the
// ConfigMap.
//
// If waitTimeout is greater than 0, the function waits for all workloads to be
// ready for the specified duration before returning.
//
// Returns an error if the client cannot be created, the ConfigMap cannot be read
// or written, kustomizations fail to apply, or workloads don't become ready within
// the timeout period.
func Kustomize(
	ctx context.Context,
	kubeClient resource.RESTClientGetter,
	fs host.FileSystem,
	options *utils.KustomizeOptions,
) error {
	if options.Kustomization == "" && !options.ForceEmbedded {
		log.Warn("Empty kustomization.")
		return nil
	}

	client, err := ClientSet(kubeClient)
	if err != nil {
		return err
	}

	cm, err := GetIkniteConfigMap(ctx, client)
	if err != nil {
		return err
	}
	if cm.Data["configured"] == configuredValueTrue && !options.ForceConfig {
		log.Info("configuration has already occurred. Use -C to force.")
		return nil
	}

	log.WithFields(log.Fields{
		"kustomization": options.Kustomization,
	}).Info("Performing configuration")

	resources, err := provision.GetBaseKustomizationResources(fs, options.Kustomization, options.ForceEmbedded)
	if err != nil {
		return fmt.Errorf("while getting kustomization resources: %w", err)
	}
	log.WithField("resourceCount", resources.Size()).Info("Applying base kustomization resources")

	ids, err := ApplyResMapWithServerSideApply(kubeClient, resources)
	if err != nil {
		return fmt.Errorf("while applying kustomization resources server side: %w", err)
	}

	cm.Data["configured"] = configuredValueTrue
	_, err = WriteIkniteConfigMap(ctx, client, cm)
	if err != nil {
		return fmt.Errorf("while writing configuration: %w", err)
	}

	log.WithFields(log.Fields{
		"kustomization": options.Kustomization,
		"resources":     ids,
	}).Info("Configuration applied")

	return nil
}

func GetIkniteConfigMap(ctx context.Context, client kubernetes.Interface) (*coreV1.ConfigMap, error) {
	cm, err := client.CoreV1().
		ConfigMaps("kube-system").
		Get(ctx, "iknite-config", metaV1.GetOptions{})
	if k8Errors.IsNotFound(err) {
		err = nil
		cm = &coreV1.ConfigMap{
			TypeMeta:   metaV1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metaV1.ObjectMeta{Name: "iknite-config", Namespace: "kube-system"},
			Immutable:  new(bool),
			Data:       map[string]string{"configured": "false"},
			BinaryData: map[string][]byte{},
		}
	}
	return cm, err
}

func WriteIkniteConfigMap(
	ctx context.Context,
	client kubernetes.Interface,
	cm *coreV1.ConfigMap,
) (*coreV1.ConfigMap, error) {
	var res *coreV1.ConfigMap
	var err error
	if cm.UID != "" {
		res, err = client.CoreV1().
			ConfigMaps("kube-system").
			Update(ctx, cm, metaV1.UpdateOptions{})
	} else {
		res, err = client.CoreV1().
			ConfigMaps("kube-system").
			Create(ctx, cm, metaV1.CreateOptions{})
	}
	if err != nil {
		return res, fmt.Errorf("failed to write iknite config map: %w", err)
	}
	return res, nil
}
