package kind

import (
	"k8s.io/client-go/kubernetes"
)

// -----------------------------------------------------------------------------
// Public Types - Cluster Interface
// -----------------------------------------------------------------------------

// Cluster objects represent a running Kind cluster on the local container runtime.
type Cluster interface {
	// Name indicates the kind cluster name of the running cluster.
	Name() string

	// Client is the configured *kubernetes.Clientset which can be used to access the Cluster's API
	Client() *kubernetes.Clientset

	// Cleanup performs any necessary teardown for the cluster
	Cleanup() error
}
