package environment

import (
	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
)

// Environment represents a full test environment including components and other
// aspects of the environment used for testing.
type Environment interface {
	// Name indicates the unique name of the testing environment
	Name() string

	// Cluster provides access to the testing environment's Kubernetes cluster.
	Cluster() cluster.Cluster
}
