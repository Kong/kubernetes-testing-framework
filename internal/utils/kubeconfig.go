package utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

// TempKubeconfig produces a kubeconfig tempfile given a cluster.
// the caller is responsible for cleaning up the file if they want it removed.
func TempKubeconfig(cluster clusters.Cluster) (*os.File, error) {
	// generate a kubeconfig from the cluster rest.Config
	kubeconfigBytes, err := generators.NewKubeConfigForRestConfig(cluster.Name(), cluster.Config())
	if err != nil {
		return nil, err
	}

	// create a tempfile to store the kubeconfig contents
	kubeconfig, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("-kubeconfig-%s", cluster.Name()))
	if err != nil {
		return nil, err
	}

	// write the contents
	c, err := kubeconfig.Write(kubeconfigBytes)
	if err != nil {
		return nil, err
	}

	// validate the file integrity
	if c != len(kubeconfigBytes) {
		return nil, fmt.Errorf("failed to write kubeconfig to %s (only %d/%d written)", kubeconfig.Name(), c, len(kubeconfigBytes))
	}

	return kubeconfig, nil
}
