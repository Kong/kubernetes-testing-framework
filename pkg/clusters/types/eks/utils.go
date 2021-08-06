package eks

import (
	"bytes"
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
	cfg, eks, err := clientForCluster(name)
	if err != nil {
		return nil, err
	}
	return &eKSCluster{
		name:   name,
		client: eks,
		cfg:    cfg,
		l:      &sync.RWMutex{},
		addons: make(clusters.Addons),
	}, nil
}

// -----------------------------------------------------------------------------
// Private Functions - Cluster Management
// -----------------------------------------------------------------------------
// only attach existing EKS cluster

// clientForCluster provides a *kubernetes.Clientset for a EKS cluster provided the cluster name.
func clientForCluster(name string) (*rest.Config, *kubernetes.Clientset, error) {
	kubeconfig := new(bytes.Buffer)

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
