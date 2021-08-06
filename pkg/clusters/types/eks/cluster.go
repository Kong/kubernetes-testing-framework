package eks

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/blang/semver/v4"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// EKS Cluster
// -----------------------------------------------------------------------------

const (
	// EKSClusterType indicates that the Kubernetes cluster was provisioned by EKS.
	EKSClusterType clusters.Type = "EKS"

	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "EKS_KEEP_CLUSTER"
)

// EKSCluster is a clusters.Cluster implementation backed by Kubernetes In Docker (EKS)
type eKSCluster struct {
	name       string
	client     *kubernetes.Clientset
	cfg        *rest.Config
	addons     clusters.Addons
	deployArgs []string
	l          *sync.RWMutex
}

// New provides a new clusters.Cluster backed by a EKS based Kubernetes Cluster.
func New(ctx context.Context) (clusters.Cluster, error) {
	return NewBuilder().Build(ctx)
}

// -----------------------------------------------------------------------------
// EKS Cluster - Cluster Implementation
// -----------------------------------------------------------------------------

func (c *eKSCluster) Name() string {
	return c.name
}

func (c *eKSCluster) Type() clusters.Type {
	return EKSClusterType
}

func (c *eKSCluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
}

func (c *eKSCluster) Cleanup(ctx context.Context) error {
	fmt.Printf("currently we do not cleanup eks cluster.")
	return nil
}

func (c *eKSCluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *eKSCluster) Config() *rest.Config {
	return c.cfg
}

func (c *eKSCluster) GetAddon(name clusters.AddonName) (clusters.Addon, error) {
	c.l.RLock()
	defer c.l.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *eKSCluster) ListAddons() []clusters.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *eKSCluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
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

func (c *eKSCluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
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
