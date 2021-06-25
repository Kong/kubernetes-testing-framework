package generators

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Public Functions - appsv1.Deployment Helpers
// -----------------------------------------------------------------------------

// NewDeploymentForContainer creates a minimal and opinionated *appsv1.Deployment object for testing based on a provided corev1.Container.
func NewDeploymentForContainer(c corev1.Container) *appsv1.Deployment {
	const labelKey = "app"
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
			Labels: map[string]string{
				labelKey: c.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: c.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: c.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{c},
				},
			},
		},
	}
}
