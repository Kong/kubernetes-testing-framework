package kind

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
	"github.com/kong/kubernetes-testing-framework/pkg/util/helm"
)

// -----------------------------------------------------------------------------
// ClusterConfiguration - Public Types
// -----------------------------------------------------------------------------

// ClusterConfigurationWithKongProxy is an object representing a Kind cluster's configuration and can effectively
// be used as a factory for kind cluster deployments. Clusters created from these configurations are
// opinionated, and will always automatically pre-deploy a Kong proxy service.
type ClusterConfigurationWithKongProxy struct {
	// DockerNetwork indicates the name of the Docker network to use for LoadBalancer IPs
	DockerNetwork string

	// DBMode indicates which database backend to use for the proxy ("off" and "postgres" are the only supported options currently)
	// Note: leaving this blank is equivalent to "off" and will deploy in DBLESS mode.
	DBMode string
}

// -----------------------------------------------------------------------------
// ClusterConfiguration - Factory Methods
// -----------------------------------------------------------------------------

// Deploy is a factory method to generate kind.Cluster objects given the configuration, with new names being selected on each deploy.
func (c *ClusterConfigurationWithKongProxy) Deploy(ctx context.Context) (cluster.Cluster, chan ProxyReadinessEvent, error) {
	return c.DeployWithName(ctx, uuid.New().String())
}

// DeployWithName is a factory method to generate kind.Cluster objects given the configuration with a custom name provided.
func (c *ClusterConfigurationWithKongProxy) DeployWithName(ctx context.Context, name string) (cluster.Cluster, chan ProxyReadinessEvent, error) {
	if c.DockerNetwork == "" {
		c.DockerNetwork = DefaultKindDockerNetwork
	}

	err := CreateCluster(name)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateCluster() failed: %w", err)
	}

	cfg, kc, err := ClientForCluster(name)
	if err != nil {
		return nil, nil, err
	}

	cluster := &kongProxyCluster{
		name:   name,
		client: kc,
		cfg:    cfg,
		l:      &sync.RWMutex{},
		addons: make(map[string]cluster.Addon),
	}

	if err := helm.DeployKongProxyOnly(name, c.DBMode); err != nil {
		return nil, nil, err
	}

	// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
	//       See: https://github.com/Kong/charts/issues/329
	if err := runUDPServiceHack(ctx, cluster); err != nil {
		return nil, nil, err
	}

	ready := make(chan ProxyReadinessEvent)
	go cluster.ProxyReadinessInformer(ctx, ready)

	return cluster, ready, nil
}
