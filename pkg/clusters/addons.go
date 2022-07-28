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

	// Dependencies indicates any addons this addon is dependent on in order
	// for operations to succeed.
	Dependencies(ctx context.Context, cluster Cluster) []AddonName

	// Deploy deploys the addon component to a provided cluster.
	// Addon implementations are responsible for waiting for their
	// own dependencies to deploy as needed.
	Deploy(ctx context.Context, cluster Cluster) error

	// Delete removes the addon component from the given cluster.
	Delete(ctx context.Context, cluster Cluster) error

	// DumpDiagnostics gathers and returns diagnostic information for an addon. Its return map is a map of string
	// filenames to file content byte slices.
	DumpDiagnostics(ctx context.Context, cluster Cluster) (map[string][]byte, error)

	// Ready is a non-blocking call which checks the status of the addon on the
	// cluster and reports any runtime.Objects which are still unresolved.
	// If all components are ready, this method will return [], true, nil.
	// If the addon has failed unrecoverably, it will provide an error.
	Ready(ctx context.Context, cluster Cluster) (waitingForObjects []runtime.Object, ready bool, err error)
}
