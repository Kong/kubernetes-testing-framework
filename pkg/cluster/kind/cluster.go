package kind

import (
	"os"
	"os/exec"
	"sync"

	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
)

// -----------------------------------------------------------------------------
// Public Functions - Cluster Management
// -----------------------------------------------------------------------------

// CreateCluster creates a new cluster using Kubernetes in Docker (KIND).
func CreateCluster(name string) error {
	// TODO: for now using CLI and outputting to stdout/stderr
	// later we should switch to using the libs.
	cmd := exec.Command("kind", "create", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DeleteKindCluster deletes an existing KIND cluster.
func DeleteKindCluster(name string) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// Public Functions - Existing Cluster
// -----------------------------------------------------------------------------

// GetExistingCluster provides a Cluster object for a given kind cluster by name.
func GetExistingCluster(name string) (cluster.Cluster, error) {
	cfg, kc, err := ClientForCluster(name)
	if err != nil {
		return nil, err
	}
	return &kongProxyCluster{
		name:   name,
		client: kc,
		cfg:    cfg,
		l:      &sync.RWMutex{},
		addons: make(map[string]cluster.Addon),
	}, nil
}
