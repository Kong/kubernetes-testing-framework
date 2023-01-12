package openshift

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	openshiftProvider "github.com/kong/kubernetes-testing-framework/pkg/clusters/types/openshift/provider"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// OpenShiftClusterType indicates that the Kubernetes cluster was provisioned by OpenShift.
	OpenShiftClusterType clusters.Type = "openshift"

	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "OPENSHIFT_KEEP_CLUSTER"
)

// Cluster is a clusters.Cluster implementation backed by OpenShift
type Cluster struct {
	*sync.RWMutex

	name              string
	client            *kubernetes.Clientset
	cfg               *rest.Config
	addons            clusters.Addons
	openshiftProvider openshiftProvider.OpenShiftProvider
}

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) Type() clusters.Type {
	return OpenShiftClusterType
}

func (c *Cluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
}

func (c *Cluster) Cleanup(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	if os.Getenv(EnvKeepCluster) == "" {
		return c.openshiftProvider.DeleteCluster(ctx)
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
	c.RLock()
	defer c.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *Cluster) ListAddons() []clusters.Addon {
	c.RLock()
	defer c.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *Cluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
	c.Lock()
	if _, ok := c.addons[addon.Name()]; ok {
		c.Unlock()
		return fmt.Errorf("addon component %s is already loaded into cluster %s", addon.Name(), c.Name())
	}
	c.addons[addon.Name()] = addon
	c.Unlock()

	return addon.Deploy(ctx, c)
}

func (c *Cluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
	c.Lock()
	defer c.Unlock()

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
