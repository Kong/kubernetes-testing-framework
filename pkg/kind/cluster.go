package kind

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	// Config provides the *rest.Config for the cluster which is convenient for initiating custom kubernetes.Clientsets.
	Config() *rest.Config

	// Cleanup obliterates the cluster and all of its resources, leaving no garbage behind, unless `KIND_KEEP_CLUSTER` is set.
	Cleanup() error
}
