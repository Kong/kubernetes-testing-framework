package k8s

import (
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Public Functions - netv1.Ingress Helpers
// -----------------------------------------------------------------------------

// NewIngressForService provides a basic and opinionated *netv1.Ingress object for the
// provided *corev1.Service to expose it via an ingress controller for testing purposes.
func NewIngressForService(ingressClass, path string, s *corev1.Service) *netv1.Ingress {
	pathPrefix := netv1.PathTypePrefix
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Name,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": ingressClass,
				"konghq.com/strip-path":       "true",
			},
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
