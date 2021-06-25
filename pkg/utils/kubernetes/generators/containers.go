package generators

import (
	corev1 "k8s.io/api/core/v1"
)

// -----------------------------------------------------------------------------
// Public Functions - corev1.Container Helpers
// -----------------------------------------------------------------------------

// NewContainer creates a minimal and opinionated corev1.Container object for testing
func NewContainer(name, image string, port int32) corev1.Container {
	return corev1.Container{
		Name:  name,
		Image: image,
		Ports: []corev1.ContainerPort{{ContainerPort: port}},
	}
}
