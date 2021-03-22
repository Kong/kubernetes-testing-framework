package kind

import (
	"bytes"
	"os"
	"os/exec"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// -----------------------------------------------------------------------------
// Public Consts & Vars
// -----------------------------------------------------------------------------

const (
	// DefaultKindDockerNetwork is the Docker network that a kind cluster uses by default.
	DefaultKindDockerNetwork = "kind"
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
// Public Functions - Helper
// -----------------------------------------------------------------------------

// ClientForCluster provides a *kubernetes.Clientset for a KIND cluster provided the cluster name.
func ClientForCluster(name string) (*kubernetes.Clientset, error) {
	kubeconfig := new(bytes.Buffer)
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", name)
	cmd.Stdout = kubeconfig
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	clientCfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig.Bytes())
	if err != nil {
		return nil, err
	}

	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restCfg)
}
