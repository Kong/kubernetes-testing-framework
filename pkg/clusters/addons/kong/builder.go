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

	// proxy options
	proxyOnly     bool
	proxyImage    string
	proxyImageTag string

	// enterprise options
	enterpriseEnabled           bool
	enterpriseLicenseSecretName string
	enterpriseSuperuserPassword string
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

// WithEnterprise deploying kong enterpise
func (b *Builder) WithEnterprise() *Builder {
	b.enterpriseEnabled = true
	return b
}

// WithImage specify docker image repo and tag
func (b *Builder) WithImage(image, tag string) *Builder {
	b.proxyImage = image
	b.proxyImageTag = tag
	return b
}

// WithLicense specify license secret name
func (b *Builder) WithLicense(license string) *Builder {
	b.enterpriseLicenseSecretName = license
	return b
}

func (b *Builder) WithKongAdminPassword(password string) *Builder {
	b.enterpriseSuperuserPassword = password
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
	if b.enterpriseEnabled {
		// if no specific name was selected for the license secret, use the default
		if b.enterpriseLicenseSecretName == "" {
			b.enterpriseLicenseSecretName = DefaultEnterpriseLicenseSecretName
		}

		// if no specific image name or tag was provided, but enterprise was enabled
		// use the default image and tag for enterprise.
		if b.proxyImage == "" {
			b.proxyImage = DefaultEnterpriseImageRepo
		}
		if b.proxyImageTag == "" {
			b.proxyImageTag = DefaultEnterpriseImageTag
		}
	}

	return &Addon{
		dbmode:                      b.dbmode,
		namespace:                   b.namespace,
		deployArgs:                  b.deployArgs,
		proxyOnly:                   b.proxyOnly,
		proxyImage:                  b.proxyImage,
		proxyImageTag:               b.proxyImageTag,
		enterpriseEnabled:           b.enterpriseEnabled,
		enterpriseLicenseSecretName: b.enterpriseLicenseSecretName,
		enterpriseSuperuserPassword: b.enterpriseSuperuserPassword,
	}
}
