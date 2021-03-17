package kind

import (
	"context"
	"os"

	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"

	"github.com/kong/kubernetes-testing-framework/pkg/metallb"
)

// -----------------------------------------------------------------------------
// ClusterConfiguration - Public Types
// -----------------------------------------------------------------------------

const (
	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "KIND_KEEP_CLUSTER"
)

// ClusterConfigurationWithKongProxy is an object representing a Kind cluster's configuration and can effectively
// be used as a factory for kind cluster deployments. Clusters created from these configurations are
// opinionated, and will always automatically pre-deploy a Kong proxy service.
type ClusterConfigurationWithKongProxy struct {
	// DockerNetwork indicates the name of the Docker network to use for LoadBalancer IPs
	DockerNetwork string

	// EnableMetalLB instructions the deployment of MetalLB to support provisioning LoadBalancer Services in the cluster.
	EnableMetalLB bool
}

// -----------------------------------------------------------------------------
// ClusterConfiguration - Factory Methods
// -----------------------------------------------------------------------------

// Deploy is a factory method to generate kind.Cluster objects given the configuration, with new names being selected on each deploy.
func (c *ClusterConfigurationWithKongProxy) Deploy(ctx context.Context) (Cluster, chan ProxyReadinessEvent, error) {
	name := uuid.New().String()

	if c.DockerNetwork == "" {
		c.DockerNetwork = DefaultKindDockerNetwork
	}

	err := CreateClusterWithKongProxy(name)
	if err != nil {
		return nil, nil, err
	}

	kc, err := ClientForCluster(name)
	if err != nil {
		return nil, nil, err
	}

	cluster := &kongProxyCluster{
		name:   name,
		client: kc,
	}

	if c.EnableMetalLB {
		cluster.enabledMetal = true
		if err := metallb.DeployMetallbForKindCluster(kc, name, c.DockerNetwork); err != nil {
			return nil, nil, err
		}
	}

	return cluster, cluster.startProxyInformer(ctx), nil
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Interface Implementation
// -----------------------------------------------------------------------------

func (c *kongProxyCluster) Name() string {
	return c.name
}

func (c *kongProxyCluster) Cleanup() error {
	if v := os.Getenv(EnvKeepCluster); v == "" {
		return DeleteKindCluster(c.name)
	}
	return nil
}

func (c *kongProxyCluster) Client() *kubernetes.Clientset {
	return c.client
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Types
// -----------------------------------------------------------------------------

type kongProxyCluster struct {
	name         string
	client       *kubernetes.Clientset
	enabledMetal bool
}
