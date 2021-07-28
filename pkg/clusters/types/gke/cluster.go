package gke

import (
	"context"
	"fmt"
	"os"
	"sync"

	container "cloud.google.com/go/container/apiv1"
	"google.golang.org/api/option"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// GKE Cluster
// -----------------------------------------------------------------------------

// gkeCluster is a clusters.Cluster implementation backed by Google Kubernetes Engine (GKE)
type gkeCluster struct {
	name      string
	project   string
	location  string
	jsonCreds []byte
	client    *kubernetes.Clientset
	cfg       *rest.Config
	addons    clusters.Addons
	l         *sync.RWMutex
}

// -----------------------------------------------------------------------------
// GKE Cluster - Cluster Implementation
// -----------------------------------------------------------------------------

func (c *gkeCluster) Name() string {
	return c.name
}

func (c *gkeCluster) Type() clusters.Type {
	return GKEClusterType
}

func (c *gkeCluster) Cleanup(ctx context.Context) error {
	c.l.Lock()
	defer c.l.Unlock()

	if os.Getenv(EnvKeepCluster) == "" {
		credsOpt := option.WithCredentialsJSON(c.jsonCreds)
		mgrc, err := container.NewClusterManagerClient(ctx, credsOpt)
		if err != nil {
			return err
		}
		defer mgrc.Close()

		_, err = deleteCluster(ctx, mgrc, c.name, c.project, c.location)
		return err
	}

	return nil
}

func (c *gkeCluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *gkeCluster) Config() *rest.Config {
	return c.cfg
}

func (c *gkeCluster) GetAddon(name clusters.AddonName) (clusters.Addon, error) {
	c.l.RLock()
	defer c.l.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *gkeCluster) ListAddons() []clusters.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *gkeCluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
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

func (c *gkeCluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
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
