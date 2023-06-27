package argocd

// Builder is a configuration tool to generate ArgoCD cluster addons.
type Builder struct {
	name      string
	namespace string
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

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	if b.namespace == "" {
		b.namespace = DefaultNamespace
	}
	return &Addon{
		name:      b.name,
		namespace: b.namespace,
	}
}
