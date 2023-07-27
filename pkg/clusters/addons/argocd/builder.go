package argocd

import "github.com/blang/semver/v4"

// Builder is a configuration tool to generate ArgoCD cluster addons.
type Builder struct {
	name      string
	namespace string
	version   *semver.Version
}

// NewBuilder provides a new Builder object for configuring ArgoCD cluster addons.
func NewBuilder() *Builder {
	return &Builder{
		name: string(AddonName),
	}
}

// WithNamespace sets the addon namespace
func (b *Builder) WithNamespace(namespace string) *Builder {
	b.namespace = namespace
	return b
}

// WithVersion configures the specific version of ArgoCD to deploy.
func (b *Builder) WithVersion(version semver.Version) *Builder {
	b.version = &version
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	if b.namespace == "" {
		b.namespace = DefaultNamespace
	}
	var version string
	if b.version == nil {
		version = "stable"
	} else {
		version = b.version.String()
	}
	return &Addon{
		name:      b.name,
		namespace: b.namespace,
		version:   version,
	}
}
