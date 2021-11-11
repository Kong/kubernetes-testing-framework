package gke

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	container "cloud.google.com/go/container/apiv1"
	"github.com/blang/semver/v4"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// NewFromExistingWithEnv provides a new clusters.Cluster backed by an existing GKE cluster,
// but allows some of the configuration to be filled in from the ENV instead of arguments.
func NewFromExistingWithEnv(ctx context.Context, name string) (clusters.Cluster, error) {
	// gather all the required env vars
	jsonCreds := os.Getenv(GKECredsVar)
	if jsonCreds == "" {
		return nil, fmt.Errorf("%s was not set", GKECredsVar)
	}
	project := os.Getenv(GKEProjectVar)
	if project == "" {
		return nil, fmt.Errorf("%s was not set", GKEProjectVar)
	}
	location := os.Getenv(GKELocationVar)
	if location == "" {
		return nil, fmt.Errorf("%s was not set", GKELocationVar)
	}

	return NewFromExisting(ctx, name, project, location, []byte(jsonCreds))
}

// NewFromExisting provides a new clusters.Cluster backed by an existing GKE cluster.
func NewFromExisting(ctx context.Context, name, project, location string, jsonCreds []byte) (clusters.Cluster, error) {
	// generate an auth token and management client
	mgrc, authToken, err := clientAuthFromCreds(ctx, jsonCreds)
	if err != nil {
		return nil, err
	}
	defer mgrc.Close()

	// get the restconfig and kubernetes client for the cluster
	cfg, client, err := clientForCluster(ctx, mgrc, authToken, name, project, location)
	if err != nil {
		return nil, err
	}

	return &gkeCluster{
		name:      name,
		project:   project,
		location:  location,
		jsonCreds: jsonCreds,
		client:    client,
		cfg:       cfg,
		addons:    make(clusters.Addons),
		l:         &sync.RWMutex{},
	}, nil
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

func (c *gkeCluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
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

func (c *gkeCluster) GetNodeAddresses(ctx context.Context) ([]string, error) {
	var addrs []string
	nodes, err := c.Client().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{}, err
	}
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				addrs = append(addrs, addr.Address)
			}
		}
	}
	return addrs, nil
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
