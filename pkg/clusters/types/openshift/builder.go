package openshift

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	openshiftProvider "github.com/kong/kubernetes-testing-framework/pkg/clusters/types/openshift/provider"
)

// Builder generates clusters.Cluster objects backed by OpenShift given
// provided configuration options.
type Builder struct {
	Name string

	addons            clusters.Addons
	openshiftProvider openshiftProvider.OpenShiftProvider
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:              uuid.NewString(),
		addons:            make(clusters.Addons),
		openshiftProvider: openshiftProvider.NewCRCProvider(),
	}
}

// Build creates and configures clients for a OpenShift-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	cluster := &Cluster{
		RWMutex:           &sync.RWMutex{},
		openshiftProvider: b.openshiftProvider,
	}

	return cluster, b.openshiftProvider.CreateCluster(ctx)
}
