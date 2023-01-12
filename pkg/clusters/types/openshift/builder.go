package openshift

import (
	"context"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	openshiftProvider "github.com/kong/kubernetes-testing-framework/pkg/clusters/types/openshift/provider"
)

// Builder generates clusters.Cluster objects backed by OpenShift given
// provided configuration options.
type Builder struct {
	Name string

	addons            clusters.Addons
	openShiftVersion  *semver.Version
	openshiftProvider openshiftProvider.OpenShiftProvider
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithOpenShiftVersion configures the Kubernetes cluster version for the Builder
// to use when building the OpenShift cluster.
func (b *Builder) WithOpenShiftVersion(version semver.Version) *Builder {
	b.openShiftVersion = &version
	return b
}

// Build creates and configures clients for a OpenShift-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	cluster := &Cluster{
		openshiftProvider: b.openshiftProvider,
	}

	return cluster, b.openshiftProvider.CreateCluster(ctx)
}
