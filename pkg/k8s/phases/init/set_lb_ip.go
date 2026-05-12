package init

// cSpell: words clientset corev metav apierrors apimachinery lbip sirupsen
import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd/phases/workflow"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/k8s"
)

const (
	setLBIPPhaseName       = "set-lb-ip"
	setLBIPAnnotation      = "config.iknite.app/outbound-ip"
	setLBIPAnnotationValue = "true"
)

func NewSetLBIPPhase() workflow.Phase {
	return workflow.Phase{
		Name:  setLBIPPhaseName,
		Short: "Set LoadBalancer ingress IPs to the outbound IP.",
		Run:   runSetLBIP,
	}
}

type setLBIPData interface {
	host.HostProvider
	IkniteClusterProvider
	ContextProvider
	RESTClientGetterProvider
	ErrGroupProvider
}

func runSetLBIP(c workflow.RunData) error {
	data, ok := c.(setLBIPData)
	if !ok {
		return fmt.Errorf("%s phase invoked with an invalid data struct", setLBIPPhaseName)
	}

	alpineHost := data.Host()

	ips := make([]string, 0)

	outboundIP, err := alpineHost.GetOutboundIP()
	if err != nil {
		return fmt.Errorf("failed to get outbound IP: %w", err)
	}
	ips = append(ips, outboundIP.String())

	clusterIp := data.IkniteCluster().Spec.Ip
	if !clusterIp.Equal(outboundIP) {
		ips = append(ips, clusterIp.String())
	}

	getter, err := data.RESTClientGetter()
	if err != nil {
		return fmt.Errorf("failed to get REST client getter: %w", err)
	}

	cs, err := k8s.ClientSet(getter)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes clientset: %w", err)
	}

	ctx := data.Context()
	data.ErrGroup().Go(func() error {
		return watchSetLBIPServices(ctx, cs, ips...)
	})

	return nil
}

func watchSetLBIPServices(
	ctx context.Context,
	cs clientset.Interface,
	outboundIPs ...string,
) error {
	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.type", string(corev1.ServiceTypeLoadBalancer)).String(),
	}

	watcher, err := cs.CoreV1().Services(metav1.NamespaceAll).Watch(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to watch services in all namespaces: %w", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type == watch.Error {
			if status, ok := event.Object.(*metav1.Status); ok {
				return fmt.Errorf("watch error: %s", status.Message)
			}
			return fmt.Errorf("unknown watch error")
		}
		if event.Type != watch.Added && event.Type != watch.Modified {
			log.WithField("eventType", event.Type).Debug("Ignoring non-added/modified service event")
			continue
		}

		service, ok := event.Object.(*corev1.Service)
		if !ok {
			log.WithField("eventObject", event.Object).Warn("Received unexpected object type in service watch")
			continue
		}

		logger := log.WithFields(log.Fields{
			"service":   service.Name,
			"namespace": service.Namespace,
			"eventType": event.Type,
		})

		logger.Info("Received service event")

		if shouldPatchServiceLBIP(service, outboundIPs) {
			logger.WithField("outboundIPs", outboundIPs).Info("Patching LoadBalancer service with outbound IP")

			if err := patchServiceLBIP(ctx, cs, service, outboundIPs); err != nil {
				log.WithError(err).WithField("service", service.Name).
					Warn("Failed to patch LoadBalancer service")
			}
		} else {
			logger.Debug("No patch needed for service")
		}
	}

	return nil
}

func shouldPatchServiceLBIP(service *corev1.Service, outboundIPs []string) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}

	annotations := service.GetAnnotations()
	if annotations == nil {
		return false
	}

	val, ok := annotations[setLBIPAnnotation]
	needsPatch := ok && val == setLBIPAnnotationValue
	if !needsPatch {
		return false
	}
	// Now check if the service already has the correct IP to avoid unnecessary patching
	if len(service.Status.LoadBalancer.Ingress) != len(outboundIPs) {
		return true
	}

	ips := make(map[string]struct{})
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		ips[ingress.IP] = struct{}{}
	}

	for _, ip := range outboundIPs {
		if _, exists := ips[ip]; !exists {
			return true
		}
	}
	return false
}

func patchServiceLBIP(
	ctx context.Context,
	cs clientset.Interface,
	service *corev1.Service,
	outboundIPs []string,
) error {
	serviceCopy := service.DeepCopy()
	// Create ports
	portStatuses := make([]corev1.PortStatus, len(service.Spec.Ports))
	for i, port := range service.Spec.Ports {
		portStatuses[i] = corev1.PortStatus{
			Port:     port.Port,
			Protocol: port.Protocol,
		}
	}
	ipMode := corev1.LoadBalancerIPModeVIP
	ingress := make([]corev1.LoadBalancerIngress, 0, len(outboundIPs))
	for _, outboundIP := range outboundIPs {
		ingress = append(ingress, corev1.LoadBalancerIngress{
			IP:     outboundIP,
			Ports:  portStatuses,
			IPMode: &ipMode,
		})
	}
	serviceCopy.Status.LoadBalancer.Ingress = ingress

	// update the service status with the new LoadBalancer IP and ports
	_, err := cs.CoreV1().Services(service.Namespace).
		UpdateStatus(ctx, serviceCopy, metav1.UpdateOptions{FieldManager: "iknite"})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithField("service", service.Name).
				Info("Service not found when patching LB IP, it may have been deleted")
			return nil
		}
		return fmt.Errorf("failed to update service status: %w", err)
	}
	log.WithFields(log.Fields{
		"service":     service.Name,
		"namespace":   service.Namespace,
		"outboundIPs": outboundIPs,
	}).Info("Successfully patched LoadBalancer service with outbound IP")

	return nil
}
