package kong

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool for Kong cluster.Addons
type Builder struct {
	namespace  string
	name       string
	deployArgs []string
	dbmode     DBMode
	proxyOnly  bool
	enterprise bool
	repo       string
	tag        string
}

// NewBuilder provides a new Builder object for configuring and generating
// Kong Addon objects which can be deployed to cluster.Clusters
func NewBuilder() *Builder {
	builder := &Builder{
		namespace:  DefaultNamespace,
		name:       DefaultDeploymentName,
		deployArgs: defaults(),
	}
	return builder.WithDBLess()
}

// WithDBLess configures the resulting Addon to deploy a DBLESS proxy backend.
func (b *Builder) WithDBLess() *Builder {
	b.dbmode = DBLESS
	return b
}

// WithPostgreSQL configures the resulting Addon to deploy a PostgreSQL proxy backend.
func (b *Builder) WithPostgreSQL() *Builder {
	b.dbmode = PostgreSQL
	return b
}

func (b *Builder) WithEnterprise(dbMode DBMode) *Builder {
	b.dbmode = dbMode
	b.enterprise = true
	return b
}

func (b *Builder) WithImage(repo, tag string) *Builder {
	b.repo = repo
	b.tag = tag
	return b
}

// WithControllerDisabled configures the Addon in proxy only mode (bring your own control plane).
func (b *Builder) WithControllerDisabled() *Builder {
	b.proxyOnly = true
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	return &Addon{
		dbmode:     b.dbmode,
		namespace:  b.namespace,
		deployArgs: b.deployArgs,
		proxyOnly:  b.proxyOnly,
	}
}
