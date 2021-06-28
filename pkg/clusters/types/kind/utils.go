package kind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Public Functions - Existing Cluster
// -----------------------------------------------------------------------------

// NewFromExisting provides a Cluster object for a given kind cluster by name.
func NewFromExisting(name string) (clusters.Cluster, error) {
	cfg, kc, err := clientForCluster(name)
	if err != nil {
		return nil, err
	}
	return &kindCluster{
		name:   name,
		client: kc,
		cfg:    cfg,
		l:      &sync.RWMutex{},
		addons: make(clusters.Addons),
	}, nil
}

// -----------------------------------------------------------------------------
// Private Functions - Cluster Management
// -----------------------------------------------------------------------------

// createCluster creates a new cluster using Kubernetes in Docker (KIND).
func createCluster(ctx context.Context, name string, extraArgs ...string) error {
	args := []string{"create", "cluster", "--name", name}
	args = append(args, extraArgs...)

	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kind", args...) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return nil
}

// deleteKindCluster deletes an existing KIND cluster.
func deleteKindCluster(ctx context.Context, name string) error {
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

// clientForCluster provides a *kubernetes.Clientset for a KIND cluster provided the cluster name.
func clientForCluster(name string) (*rest.Config, *kubernetes.Clientset, error) {
	kubeconfig := new(bytes.Buffer)
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", name)
	cmd.Stdout = kubeconfig
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, err
	}

	clientCfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig.Bytes())
	if err != nil {
		return nil, nil, err
	}

	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	return cfg, clientset, err
}
