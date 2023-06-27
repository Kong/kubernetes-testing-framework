package kongargo

// Builder is a configuration tool to generate ArgoCD cluster addons.
type Builder struct {
	name      string
	namespace string
	version   string
	project   string
	release   string
	appName   string
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

// WithProject sets the addon project
func (b *Builder) WithProject(project string) *Builder {
	b.project = project
	return b
}

// WithVersion sets the addon version
func (b *Builder) WithVersion(version string) *Builder {
	b.version = version
	return b
}

// WithRelease sets the addon release
func (b *Builder) WithRelease(release string) *Builder {
	b.release = release
	return b
}

// WithAppName sets the addon application name
func (b *Builder) WithAppName(appName string) *Builder {
	b.appName = appName
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	if b.namespace == "" {
		b.namespace = DefaultNamespace
	}
	if b.project == "" {
		b.project = defaultProject
	}
	if b.version == "" {
		b.version = defaultVersion
	}
	if b.release == "" {
		b.release = defaultRelease
	}
	if b.appName == "" {
		b.appName = defaultAppName
	}
	return &Addon{
		name:      b.name,
		namespace: b.namespace,
		project:   b.project,
		version:   b.version,
		release:   b.release,
		appName:   b.appName,
	}
}
