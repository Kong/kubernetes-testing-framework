package runbooks

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kong/kubernetes-testing-framework/pkg/generators/k8s"
)

// DeployIngressForContainer is a runbook which simplifies creating a Deployment, Service, and Ingress resource given a container specification for which they should serve.
// TODO: This is opionated and ultimately expects that the Ingress controller is going to be kong, and that `strip-path` should be used.
func DeployIngressForContainer(kc *kubernetes.Clientset, ingressClass, ingressPath string, container corev1.Container) error {
	ctx := context.Background()
	opts := metav1.CreateOptions{}

	deployment := k8s.NewDeploymentForContainer(container)
	_, err := kc.AppsV1().Deployments("default").Create(ctx, deployment, opts)
	if err != nil {
		return err
	}

	service := k8s.NewServiceForDeployment(deployment, corev1.ServiceTypeClusterIP)
	_, err = kc.CoreV1().Services("default").Create(ctx, service, opts)
	if err != nil {
		return err
	}

	ingress := k8s.NewIngressForService(ingressPath, map[string]string{
		"kubernetes.io/ingress.class": ingressClass,
		"konghq.com/strip-path":       "true",
	}, service)
	_, err = kc.NetworkingV1().Ingresses("default").Create(ctx, ingress, opts)

	return err
}
