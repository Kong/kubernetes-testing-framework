package generators

import (
	"github.com/blang/semver/v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// -----------------------------------------------------------------------------
// Public Functions - netv1.Ingress Helpers
// -----------------------------------------------------------------------------

// NewIngressForServiceWithClusterVersion provides an Ingress record for the provided service, but uses a provided
// Kubernetes cluster version to determine which Ingress version to provide (provides latest available for release).
func NewIngressForServiceWithClusterVersion(kubernetesVersion semver.Version, path string, annotations map[string]string, s *corev1.Service) runtime.Object {
	if kubernetesVersion.Major < 2 && kubernetesVersion.Minor < 19 {
		return NewLegacyIngressForService(path, annotations, s)
	}
	return NewIngressForService(path, annotations, s)
}

// NewIngressForService provides a basic and opinionated *netv1.Ingress object for the provided *corev1.Service to expose it via an ingress controller for testing purposes.
func NewIngressForService(path string, annotations map[string]string, s *corev1.Service) *netv1.Ingress {
	pathPrefix := netv1.PathTypePrefix
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathPrefix,
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: s.Name,
											Port: netv1.ServiceBackendPort{
												Number: s.Spec.Ports[0].Port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// NewLegacyIngressForService provides a basic and opinionated *netv1beta1.Ingress object for the provided *corev1.Service to expose it via an ingress controller for testing purposes.
func NewLegacyIngressForService(path string, annotations map[string]string, s *corev1.Service) *netv1beta1.Ingress {
	pathPrefix := netv1beta1.PathTypePrefix
	return &netv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Annotations: annotations,
		},
		Spec: netv1beta1.IngressSpec{
			Rules: []netv1beta1.IngressRule{
				{
					IngressRuleValue: netv1beta1.IngressRuleValue{
						HTTP: &netv1beta1.HTTPIngressRuleValue{
							Paths: []netv1beta1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathPrefix,
									Backend: netv1beta1.IngressBackend{
										ServiceName: s.Name,
										ServicePort: intstr.FromInt(int(s.Spec.Ports[0].Port)),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// NewIngressForContainerWithDeploymentAndService generates a Deployment, Service, and Ingress given a container.
// The idea is that if you have a container that provides an HTTP endpoint, this function can be used to generate
// everything you need to deploy to the cluster to start accessing that HTTP server from outside the cluster via Ingress.
// This effectively just compiles together multiple generators for convenience, look at the individual generators
// used here if you're looking for something more granular.
func NewIngressForContainerWithDeploymentAndService(
	kubernetesVersion semver.Version,
	c corev1.Container,
	serviceType corev1.ServiceType,
	annotations map[string]string,
	path string,
) (*appsv1.Deployment, *corev1.Service, runtime.Object) {
	deployment := NewDeploymentForContainer(c)
	service := NewServiceForDeployment(deployment, serviceType)
	ingress := NewIngressForServiceWithClusterVersion(kubernetesVersion, path, annotations, service)
	return deployment, service, ingress
}
