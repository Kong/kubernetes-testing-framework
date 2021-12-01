package registry

import (
	"github.com/blang/semver/v4"
)

// -----------------------------------------------------------------------------
// Registry Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool to generate Registry cluster addons.
type Builder struct {
	name                    string
	registryVersion         semver.Version
	serviceTypeLoadBalancer bool
}

// NewBuilder provides a new Builder object for configuring Registry cluster addons.
func NewBuilder() *Builder {
	return &Builder{
		name: string(AddonName),
	}
}

// WithVersion configures the specific version of Registry which should be deployed.
func (b *Builder) WithVersion(version semver.Version) *Builder {
	b.registryVersion = version
	return b
}

// WithServiceTypeLoadBalancer configures the Registry's Kubernetes service to be
// of type LoadBalancer to support access from outside the cluster network.
func (b *Builder) WithServiceTypeLoadBalancer() *Builder {
	b.serviceTypeLoadBalancer = true
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	return &Addon{
		name:                    b.name,
		registryVersion:         &b.registryVersion,
		serviceTypeLoadBalancer: b.serviceTypeLoadBalancer,
	}
}
