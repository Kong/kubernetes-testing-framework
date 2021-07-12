package kind

import (
	"context"
	"fmt"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// Builder generates clusters.Cluster objects backed by Kind given
// provided configuration options.
type Builder struct {
	Name string

	addons         clusters.Addons
	clusterVersion *semver.Version
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to use for the cluster.
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithClusterVersion configures the Kubernetes cluster version for the Builder
// to use when building the Kind cluster.
func (b *Builder) WithClusterVersion(version semver.Version) *Builder {
	b.clusterVersion = &version
	return b
}

// Build creates and configures clients for a Kind-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	deployArgs := make([]string, 0)
	if b.clusterVersion != nil {
		deployArgs = append(deployArgs, "--image", "kindest/node:v"+b.clusterVersion.String())
	}

	if err := createCluster(ctx, b.Name, deployArgs...); err != nil {
		return nil, fmt.Errorf("failed to create cluster %s: %w", b.Name, err)
	}

	cfg, kc, err := clientForCluster(b.Name)
	if err != nil {
		return nil, err
	}

	return &kindCluster{
		name:       b.Name,
		client:     kc,
		cfg:        cfg,
		addons:     make(clusters.Addons),
		deployArgs: deployArgs,
		l:          &sync.RWMutex{},
	}, nil
}
