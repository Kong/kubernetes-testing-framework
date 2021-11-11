package kind

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/blang/semver/v4"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Kind Cluster
// -----------------------------------------------------------------------------

const (
	// KindClusterType indicates that the Kubernetes cluster was provisioned by Kind.
	KindClusterType clusters.Type = "kind"

	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "KIND_KEEP_CLUSTER"

	// DefaultKindDockerNetwork is the Docker network that a kind cluster uses by default.
	DefaultKindDockerNetwork = "kind"
)

// kindCluster is a clusters.Cluster implementation backed by Kubernetes In Docker (KIND)
type kindCluster struct {
	name       string
	client     *kubernetes.Clientset
	cfg        *rest.Config
	addons     clusters.Addons
	deployArgs []string
	l          *sync.RWMutex
}

// New provides a new clusters.Cluster backed by a Kind based Kubernetes Cluster.
func New(ctx context.Context) (clusters.Cluster, error) {
	return NewBuilder().Build(ctx)
}

// -----------------------------------------------------------------------------
// Kind Cluster - Cluster Implementation
// -----------------------------------------------------------------------------

func (c *kindCluster) Name() string {
	return c.name
}

func (c *kindCluster) Type() clusters.Type {
	return KindClusterType
}

func (c *kindCluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
}

func (c *kindCluster) Cleanup(ctx context.Context) error {
	c.l.Lock()
	defer c.l.Unlock()

	if os.Getenv(EnvKeepCluster) == "" {
		return deleteKindCluster(ctx, c.name)
	}

	return nil
}

func (c *kindCluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *kindCluster) GetNodeAddresses(ctx context.Context) ([]string, error) {
	var addrs []string
	nodes, err := c.Client().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{}, err
	}
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			// for KIND, these are the actual local routeable IPs. there is no NodeExternalIP
			if addr.Type == corev1.NodeInternalIP {
				addrs = append(addrs, addr.Address)
			}
		}
	}
	return addrs, nil
}

func (c *kindCluster) Config() *rest.Config {
	return c.cfg
}

func (c *kindCluster) GetAddon(name clusters.AddonName) (clusters.Addon, error) {
	c.l.RLock()
	defer c.l.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *kindCluster) ListAddons() []clusters.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *kindCluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.addons[addon.Name()]; ok {
		return fmt.Errorf("addon component %s is already loaded into cluster %s", addon.Name(), c.Name())
	}

	if err := addon.Deploy(ctx, c); err != nil {
		return err
	}

	c.addons[addon.Name()] = addon

	return nil
}

func (c *kindCluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.addons[addon.Name()]; !ok {
		return nil
	}

	if err := addon.Delete(ctx, c); err != nil {
		return err
	}

	delete(c.addons, addon.Name())

	return nil
}
