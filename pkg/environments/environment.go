package environments

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Public Types - Testing Environments
// -----------------------------------------------------------------------------

// Environment represents a full test environment including components and other
// aspects of the environment used for testing.
type Environment interface {
	// Name indicates the unique name of the testing environment
	Name() string

	// Cluster provides access to the testing environment's Kubernetes cluster.
	Cluster() clusters.Cluster

	// Cleanup performs teardown and cleanup on all cluster components
	Cleanup(ctx context.Context) error

	// Ready indicates when the environment is ready and fully deployed,
	// or if errors occurred during provisioning of components.
	Ready(ctx context.Context) ([]runtime.Object, bool, error)

	// WaitForReady provides a nonblocking channel which can be used to wait
	// for readiness of the Environment. The channel has a timeout that is
	// based on the underlying env.Cluster.Type() and if no error is received
	// the caller may assume all runtime objects are resolved.
	WaitForReady(ctx context.Context) chan error
}
