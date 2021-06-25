package clusters

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// -----------------------------------------------------------------------------
// Public Types - Cluster Addons
// -----------------------------------------------------------------------------

// AddonName indicates a unique name for Addons which can be deployed to Clusters.
type AddonName string

// Addons is a map type for collecting a list of unique Addons.
type Addons map[AddonName]Addon

// Addon is a loadable component to extend the functionality of a Cluster.
type Addon interface {
	// Name indicates the unique name of the Addon
	Name() AddonName

	// Deploy deploys the addon component to a provided cluster.
	Deploy(ctx context.Context, cluster Cluster) error

	// Delete removes the addon component from the given cluster.
	Delete(ctx context.Context, cluster Cluster) error

	// Ready is a non-blocking call which checks the status of the addon on the
	// cluster and reports any runtime.Objects which are still unresolved.
	// If all components are ready, this method will return [], true, nil.
	// If the addon has failed unrecoverably, it will provide an error.
	Ready(ctx context.Context, cluster Cluster) (waitingForObjects []runtime.Object, ready bool, err error)
}
