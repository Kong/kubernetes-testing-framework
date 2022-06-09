package kong

import (
	"io"

	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool for Kong cluster.Addons
type Builder struct {
	logger *logrus.Logger

	// kubernetes and helm chart related configuration options
	namespace  string
	name       string
	deployArgs []string

	// ingress controller configuration options
	ingressControllerDisabled bool
	ingressControllerImage    string
	ingressControllerImageTag string

	// proxy server general configuration options
	proxyAdminServiceTypeLoadBalancer bool
	proxyDBMode                       DBMode
	proxyImage                        string
	proxyImageTag                     string

	// proxy server enterprise mode configuration options
	proxyEnterpriseEnabled            bool
	proxyEnterpriseSuperAdminPassword string
	proxyEnterpriseLicenseJSON        string
}

// NewBuilder provides a new Builder object for configuring and generating
// Kong Addon objects which can be deployed to cluster.Clusters
func NewBuilder() *Builder {
	builder := &Builder{
		namespace:  DefaultNamespace,
		name:       DefaultDeploymentName,
		deployArgs: []string{},
	}
	return builder.WithDBLess()
}

// WithLogger adds a logger that will provide extra information about the build step
// of the addon at various configured log levels.
func (b *Builder) WithLogger(logger *logrus.Logger) *Builder {
	b.logger = logger
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	// if no logger was provided provide a nil logger as a default
	if b.logger == nil {
		b.logger = &logrus.Logger{Out: io.Discard}
	}

	return &Addon{
		logger: b.logger,

		namespace:  b.namespace,
		name:       b.name,
		deployArgs: b.deployArgs,

		ingressControllerDisabled: b.ingressControllerDisabled,
		ingressControllerImage:    b.ingressControllerImage,
		ingressControllerImageTag: b.ingressControllerImageTag,

		proxyAdminServiceTypeLoadBalancer: b.proxyAdminServiceTypeLoadBalancer,
		proxyDBMode:                       b.proxyDBMode,
		proxyImage:                        b.proxyImage,
		proxyImageTag:                     b.proxyImageTag,

		proxyEnterpriseEnabled:            b.proxyEnterpriseEnabled,
		proxyEnterpriseLicenseJSON:        b.proxyEnterpriseLicenseJSON,
		proxyEnterpriseSuperAdminPassword: b.proxyEnterpriseSuperAdminPassword,
	}
}

// -----------------------------------------------------------------------------
// Kong Proxy Configuration Options
// -----------------------------------------------------------------------------

// WithControllerDisabled configures the Addon in proxy only mode (bring your own control plane).
func (b *Builder) WithControllerDisabled() *Builder {
	b.ingressControllerDisabled = true
	return b
}

// -----------------------------------------------------------------------------
// Kong Proxy Configuration Options
// -----------------------------------------------------------------------------

// WithDBLess configures the resulting Addon to deploy a DBLESS proxy backend.
func (b *Builder) WithDBLess() *Builder {
	b.proxyDBMode = DBLESS
	return b
}

// WithPostgreSQL configures the resulting Addon to deploy a PostgreSQL proxy backend.
func (b *Builder) WithPostgreSQL() *Builder {
	b.proxyDBMode = PostgreSQL
	return b
}

// WithProxyImage configures the container image name and tag for the Kong proxy.
func (b *Builder) WithProxyImage(repo, tag string) *Builder {
	b.proxyImage = repo
	b.proxyImageTag = tag
	return b
}

// WithControllerImage configures the ingress controller container image name and tag.
func (b *Builder) WithControllerImage(repo, tag string) *Builder {
	b.ingressControllerImage = repo
	b.ingressControllerImageTag = tag
	return b
}

// -----------------------------------------------------------------------------
// Kong Proxy Enterprise Configuration Options
// -----------------------------------------------------------------------------

// WithProxyAdminServiceTypeLoadBalancer sets the Kong proxy's admin API's
// Kubernetes Service type to a "LoadBalancer" type for access outside of
// the cluster.
//
// WARNING: Keep in mind that depending on your cluster provider and configuration
// using this option may expose your admin api endpoint to the internet.
func (b *Builder) WithProxyAdminServiceTypeLoadBalancer() *Builder {
	b.proxyAdminServiceTypeLoadBalancer = true
	return b
}

// WithProxyEnterpriseEnabled configures the resulting Addon to deploy the enterprise version of
// the Kong proxy.
// See: https://docs.konghq.com/enterprise/
func (b *Builder) WithProxyEnterpriseEnabled(licenseJSON string) *Builder {
	b.proxyEnterpriseLicenseJSON = licenseJSON
	b.proxyEnterpriseEnabled = true
	if b.proxyImage == "" {
		b.proxyImage = DefaultEnterpriseImageRepo
	}
	if b.proxyImageTag == "" {
		b.proxyImageTag = DefaultEnterpriseImageTag
	}
	return b
}

// WithProxyEnterpriseSuperAdminPassword specify kong admin password
func (b *Builder) WithProxyEnterpriseSuperAdminPassword(password string) *Builder {
	b.proxyEnterpriseSuperAdminPassword = password
	return b
}
