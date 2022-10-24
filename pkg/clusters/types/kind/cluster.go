package kind

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

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

// Cluster is a clusters.Cluster implementation backed by Kubernetes In Docker (KIND)
type Cluster struct {
	name       string
	client     *kubernetes.Clientset
	cfg        *rest.Config
	addons     clusters.Addons
	deployArgs []string
	l          *sync.RWMutex
}

// New provides a new clusters.Cluster backed by a Kind based Kubernetes Cluster.
func New(ctx context.Context) (*Cluster, error) {
	cluster, err := NewBuilder().Build(ctx)
	return cluster.(*Cluster), err
}

// -----------------------------------------------------------------------------
// Kind Cluster - Cluster Implementation
// -----------------------------------------------------------------------------

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) Type() clusters.Type {
	return KindClusterType
}

func (c *Cluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
}

func (c *Cluster) Cleanup(ctx context.Context) error {
	c.l.Lock()
	defer c.l.Unlock()

	if os.Getenv(EnvKeepCluster) == "" {
		return deleteKindCluster(ctx, c.name)
	}

	return nil
}

func (c *Cluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *Cluster) Config() *rest.Config {
	return c.cfg
}

func (c *Cluster) GetAddon(name clusters.AddonName) (clusters.Addon, error) {
	c.l.RLock()
	defer c.l.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *Cluster) ListAddons() []clusters.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *Cluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
	c.l.Lock()
	if _, ok := c.addons[addon.Name()]; ok {
		c.l.Unlock()
		return fmt.Errorf("addon component %s is already loaded into cluster %s", addon.Name(), c.Name())
	}
	c.addons[addon.Name()] = addon
	c.l.Unlock()

	return addon.Deploy(ctx, c)
}

func (c *Cluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
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

// DumpDiagnostics produces diagnostics data for the cluster at a given time.
// It uses the provided meta string to write to meta.txt file which will allow
// for diagnostics identification.
// It returns the path to directory containing all the diagnostic files and an error.
func (c *Cluster) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	return clusters.DumpDiagnostics(ctx, c, meta)
}
