package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Public Functions - corev1.Service Helpers
// -----------------------------------------------------------------------------

// NewServiceForDeployment provides a basic and opinionated service to expose the provided *appsv1.Deployment for testing purposes.
func NewServiceForDeployment(d *appsv1.Deployment, serviceType corev1.ServiceType) *corev1.Service {
	svcPorts := []corev1.ServicePort{}
	for _, p := range d.Spec.Template.Spec.Containers[0].Ports {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:     p.Name,
			Protocol: p.Protocol,
			Port:     p.ContainerPort,
		})
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: d.Name,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: d.Spec.Selector.MatchLabels,
			Ports:    svcPorts,
		},
	}
}
