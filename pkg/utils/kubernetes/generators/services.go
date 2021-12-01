package generators

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// -----------------------------------------------------------------------------
// Public Functions - corev1.Service Helpers
// -----------------------------------------------------------------------------

// NewServiceForDeployment provides a minimal and opinionated service to expose the provided *appsv1.Deployment
// the ports and targetports from the service will match the container ports.
func NewServiceForDeployment(d *appsv1.Deployment, serviceType corev1.ServiceType) *corev1.Service {
	return NewServiceForDeploymentWithMappedPorts(d, serviceType, make(map[int32]int32))
}

// NewServiceForDeployment provides a minimal and opinionated service to expose the provided *appsv1.Deployment
// but accepts an optional map of container ports to service ports to allow custom port declarations rather than
// mapping ports according to the container ports. The keys on the portmap are the container ports, the values
// are the service ports you want them mapped to. The portmap doesn't have to have a mapping for every port you
// can leave any blank you want, if you do they will be defaulted to the containerports found in the deployment.
func NewServiceForDeploymentWithMappedPorts(d *appsv1.Deployment, serviceType corev1.ServiceType, portMap map[int32]int32) *corev1.Service {
	// map all the container ports
	svcPorts := []corev1.ServicePort{}
	for _, p := range d.Spec.Template.Spec.Containers[0].Ports {
		mappedPort, ok := portMap[p.ContainerPort]
		if ok {
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:       p.Name,
				Protocol:   p.Protocol,
				Port:       mappedPort,
				TargetPort: intstr.FromInt(int(p.ContainerPort)),
			})
		} else {
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:     p.Name,
				Protocol: p.Protocol,
				Port:     p.ContainerPort,
			})
		}
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
