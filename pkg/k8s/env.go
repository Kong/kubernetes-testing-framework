package k8s

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// OverrideEnvVar is a utility function to override a specific environment variable in a corev1.Container.
// Errors occur if the Environment does not include the desired variable, as opposed to adding it.
func OverrideEnvVar(container *corev1.Container, key, val string) (original *corev1.EnvVar, err error) {
	newEnv := make([]corev1.EnvVar, 0, len(container.Env))
	for _, envvar := range container.Env {
		// override any existing value with our custom configuration
		if envvar.Name == key {
			// save the original configuration so we can put it back after we finish testing
			original = envvar.DeepCopy()
			envvar.Value = val
		}
		newEnv = append(newEnv, envvar)
	}

	if original == nil {
		err = fmt.Errorf("could not override env var: %s was not present on container %s", key, container.Name)
	}

	container.Env = newEnv
	return
}
