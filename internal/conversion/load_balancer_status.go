package conversion

import (
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
)

func NetV1ToCoreV1LoadBalancerStatus(in netv1.IngressLoadBalancerStatus) *corev1.LoadBalancerStatus {
	out := &corev1.LoadBalancerStatus{}
	for _, i := range in.Ingress {
		ports := make([]corev1.PortStatus, 0, len(i.Ports))
		for _, p := range i.Ports {
			ports = append(ports, corev1.PortStatus{
				Port:     p.Port,
				Protocol: p.Protocol,
				Error:    p.Error,
			})
		}
		out.Ingress = append(out.Ingress, corev1.LoadBalancerIngress{
			IP:       i.IP,
			Hostname: i.Hostname,
			Ports:    ports,
		})
	}
	return out
}

func NetV1beta1ToCoreV1LoadBalancerStatus(in netv1beta1.IngressLoadBalancerStatus) *corev1.LoadBalancerStatus {
	out := &corev1.LoadBalancerStatus{}
	for _, i := range in.Ingress {
		ports := make([]corev1.PortStatus, 0, len(i.Ports))
		for _, p := range i.Ports {
			ports = append(ports, corev1.PortStatus{
				Port:     p.Port,
				Protocol: p.Protocol,
				Error:    p.Error,
			})
		}
		out.Ingress = append(out.Ingress, corev1.LoadBalancerIngress{
			IP:       i.IP,
			Hostname: i.Hostname,
			Ports:    ports,
		})
	}
	return out
}

func ExtV1beta1ToCoreV1LoadBalancerStatus(in extv1beta1.IngressLoadBalancerStatus) *corev1.LoadBalancerStatus {
	out := &corev1.LoadBalancerStatus{}
	for _, i := range in.Ingress {
		ports := make([]corev1.PortStatus, 0, len(i.Ports))
		for _, p := range i.Ports {
			ports = append(ports, corev1.PortStatus{
				Port:     p.Port,
				Protocol: p.Protocol,
				Error:    p.Error,
			})
		}
		out.Ingress = append(out.Ingress, corev1.LoadBalancerIngress{
			IP:       i.IP,
			Hostname: i.Hostname,
			Ports:    ports,
		})
	}
	return out
}
