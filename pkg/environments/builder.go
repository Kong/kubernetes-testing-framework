package environments

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// Environment Builder
// -----------------------------------------------------------------------------

// Builder is a toolkit for building a new test Environment.
type Builder struct {
	Name string

	addons            clusters.Addons
	existingCluster   clusters.Cluster
	kubernetesVersion *semver.Version
}

// NewBuilder generates a new empty Builder for creating Environments.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to provide the testing environment
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithAddons includes any provided Addon components in the cluster
// after the cluster is deployed.
func (b *Builder) WithAddons(addons ...clusters.Addon) *Builder {
	for _, addon := range addons {
		b.addons[addon.Name()] = addon
	}
	return b
}

// WithExistingCluster causes the resulting environment to re-use an existing
// clusters.Cluster instead of creating a new one.
func (b *Builder) WithExistingCluster(cluster clusters.Cluster) *Builder {
	b.existingCluster = cluster
	return b
}

// WithKubernetesVersion indicates which Kubernetes version to deploy clusters
// with, if the caller wants something other than the default.
func (b *Builder) WithKubernetesVersion(version semver.Version) *Builder {
	b.kubernetesVersion = &version
	return b
}

// Build is a blocking call to construct the configured Environment and it's
// underlying Kubernetes cluster. The amount of time that it blocks depends
// entirely on the underlying clusters.Cluster implementation that was requested.
func (b *Builder) Build(ctx context.Context) (Environment, error) {
	var cluster clusters.Cluster

	if b.existingCluster == nil {
		var err error
		builder := kind.NewBuilder().WithName(b.Name)
		if b.kubernetesVersion != nil {
			builder.WithClusterVersion(*b.kubernetesVersion)
		}
		cluster, err = builder.Build(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		if b.kubernetesVersion != nil {
			return nil, fmt.Errorf("can't provide kubernetes version when using an existing cluster")
		}
		cluster = b.existingCluster
	}


	return &environment{
		name:    b.Name,
		cluster: cluster,
	}, nil
}
