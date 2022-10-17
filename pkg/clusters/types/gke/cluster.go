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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// GKE Cluster
// -----------------------------------------------------------------------------

// Cluster is a clusters.Cluster implementation backed by Google Kubernetes Engine (GKE)
type Cluster struct {
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
func NewFromExistingWithEnv(ctx context.Context, name string) (*Cluster, error) {
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
func NewFromExisting(ctx context.Context, name, project, location string, jsonCreds []byte) (*Cluster, error) {
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

	return &Cluster{
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

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) Type() clusters.Type {
	return GKEClusterType
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
