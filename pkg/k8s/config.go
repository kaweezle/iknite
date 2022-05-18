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

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kaweezle/iknite/pkg/constants"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Config api.Config

// LoadFromFile loads the configuration from the file specified by filename.
func LoadFromFile(filename string) (*Config, error) {
	_config, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return nil, err
	}
	config := (*Config)(_config)
	return config, nil
}

// LoadFromDefault loads the configuration from the default admin.conf file,
// usually located at /etc/kubernetes/admin.conf.
func LoadFromDefault() (*Config, error) {
	return LoadFromFile(constants.KubernetesAdminConfig)
}

// RenameConfig changes the name of the cluster and the context from the
// default (kubernetes) to newName in c.
func (c *Config) RenameConfig(newName string) *Config {
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

// Client returns a clientset for config.
func (config *Config) Client() (client *kubernetes.Clientset, err error) {
	clientconfig := clientcmd.NewDefaultClientConfig(api.Config(*config), nil)
	var rest *rest.Config
	rest, err = clientconfig.ClientConfig()
	if err != nil {
		return
	}
	client, err = kubernetes.NewForConfig(rest)
	return
}

// CheckClusterRunning checks that the cluster is running by requesting the
// API server /readyz endpoint. It checks retries times and waits for waittime
// seconds between each check. It needs at least okresponses good responses from
// the server.
func (config *Config) CheckClusterRunning(retries, okresponses, waittime int) error {

	client, err := config.Client()
	if err != nil {
		return err
	}

	oktries := 0
	query := client.Discovery().RESTClient().Get().AbsPath("/readyz")
	for retries > 0 {
		content, err := query.DoRaw(context.Background())
		if err == nil {
			contentStr := string(content)
			if contentStr != "ok" {
				err = fmt.Errorf("cluster health API returned: %s", contentStr)
				log.WithError(err).Debug("Bad response")
			} else {
				oktries = oktries + 1
				log.WithField("oktries", oktries).Trace("Ok response from server")
				if oktries == okresponses {
					break
				}
			}
		} else {
			log.WithError(err).Debug("while querying cluster readyness")
		}

		retries = retries - 1
		if retries == 0 {
			log.Trace("No more retries left.")
			return err
		} else {
			log.WithFields(log.Fields{
				"err":       err,
				"wait_time": waittime}).Debug("Waiting...")
			time.Sleep(time.Duration(waittime) * time.Second)
		}
	}

	return err
}

// WriteToFile writes the config configuration to the file pointed by filename.
// it returns the appropriate error in case of failure.
func (config *Config) WriteToFile(filename string) error {
	return clientcmd.WriteToFile(*(*api.Config)(config), filename)
}

// RestartProxy restarts kube-proxy after config has been updated. This needs to
// be done after an IP address change.
// The restart method is taken from kubectl: https://github.com/kubernetes/kubectl/blob/652881798563c00c1895ded6ced819030bfaa4d7/pkg/polymorphichelpers/objectrestarter.go#L81
func (config *Config) RestartProxy() (err error) {

	var client *kubernetes.Clientset
	if client, err = config.Client(); err != nil {
		return
	}

	ctx := context.Background()

	var ds *appsv1.DaemonSet
	if ds, err = client.AppsV1().DaemonSets("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{}); err != nil {
		return
	}

	if ds.Spec.Template.ObjectMeta.Annotations == nil {
		ds.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	ds.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = client.AppsV1().DaemonSets("kube-system").Update(ctx, ds, metav1.UpdateOptions{})
	return
}

func GetIkniteConfigMap(client *kubernetes.Clientset) (cm *corev1.ConfigMap, err error) {
	cm, err = client.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "iknite-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		err = nil
		cm = &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "iknite-config", Namespace: "kube-system"},
			Immutable:  new(bool),
			Data:       map[string]string{"configured": "false"},
			BinaryData: map[string][]byte{},
		}
	}
	return
}

func WriteIkniteConfigMap(client *kubernetes.Clientset, cm *corev1.ConfigMap) (res *corev1.ConfigMap, err error) {
	if cm.UID != "" {
		res, err = client.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), cm, metav1.UpdateOptions{})
	} else {
		res, err = client.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), cm, metav1.CreateOptions{})
	}

	return
}
