package k8s

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// IsPodReady returns false if the Pod Status is nil
func IsPodReady(pod *v1.Pod) bool {
	condition := getPodReadyCondition(&pod.Status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

func getPodReadyCondition(status *v1.PodStatus) *v1.PodCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == v1.PodReady {
			return &status.Conditions[i]
		}
	}
	return nil
}

func GetPodsSeparatedByStatus(pods []v1.Pod) (active, unready, stopped []*v1.Pod) {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case v1.PodRunning:
			if IsPodReady(&pod) {
				active = append(active, &pod)
			} else {
				unready = append(unready, &pod)
			}
		case v1.PodPending, v1.PodUnknown:
			unready = append(unready, &pod)
		default:
			stopped = append(stopped, &pod)
		}
	}

	return active, unready, stopped
}

func GetPodStatus(clientset kubernetes.Interface) (active, unready, stopped []*v1.Pod, err error) {
	var list *v1.PodList
	if list, err = clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{}); err != nil {
		return
	}
	active, unready, stopped = GetPodsSeparatedByStatus(list.Items)
	return
}

func arePodsReady(c kubernetes.Interface, minimumPodsReady int) wait.ConditionFunc {
	return func() (bool, error) {

		active, unready, stopped, err := GetPodStatus(c)
		if err != nil {
			return false, err
		}
		log.WithFields(log.Fields{
			"active":  len(active),
			"unready": len(unready),
			"stopped": len(stopped),
		}).Infof("active: %d, unready: %d, stopped:%d", len(active), len(unready), len(stopped))
		if len(stopped) > 0 {
			return false, fmt.Errorf("stopped pods: %d", len(stopped))
		}
		return len(active) > minimumPodsReady && len(unready) == 0, nil
	}
}

func waitForPodsReady(c kubernetes.Interface, timeout time.Duration, minimumPodsReady int) error {
	return wait.Poll(time.Second*time.Duration(2), timeout, arePodsReady(c, minimumPodsReady))
}

func (config *Config) WaitForCluster(timeout time.Duration, minimumPodsReady int) (err error) {
	log.Info("Wait for kubernetes...")

	var client *kubernetes.Clientset
	client, err = config.Client()
	if err != nil {
		return
	}

	err = waitForPodsReady(client, timeout, minimumPodsReady)

	if err != nil {
		log.WithError(err).Error("Kubernetes not ready")
	} else {
		log.WithError(err).Info("Cluster ready")
	}

	return
}
