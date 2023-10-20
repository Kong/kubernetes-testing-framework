package kong

import (
	"io"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool for Kong cluster.Addons
type Builder struct {
	logger *logrus.Logger

	// kubernetes and helm chart related configuration options
	namespace    string
	name         string
	deployArgs   []string
	chartVersion string

	// ingress controller configuration options
	ingressControllerDisabled bool
	ingressControllerImage    string
	ingressControllerImageTag string

	// proxy server general configuration options
	proxyAdminServiceTypeLoadBalancer bool
	proxyDBMode                       DBMode
	proxyImage                        string
	proxyImageTag                     string
	proxyPullSecret                   pullSecret
	proxyLogLevel                     string
	proxyServiceType                  corev1.ServiceType
	proxyEnvVars                      map[string]string
	proxyReadinessProbePath           string

	// ports
	httpNodePort  int
	adminNodePort int

	// proxy server enterprise mode configuration options
	proxyEnterpriseEnabled            bool
	proxyEnterpriseSuperAdminPassword string
	proxyEnterpriseLicenseJSON        string
	// additionalValues stores values that are set during installing by helm.
	// for each key-value pair, an argument `--set <key>=<value>` is added.
	additionalValues map[string]string
}

// NewBuilder provides a new Builder object for configuring and generating
// Kong Addon objects which can be deployed to cluster.Clusters
func NewBuilder() *Builder {
	builder := &Builder{
		namespace:        DefaultNamespace,
		name:             DefaultDeploymentName,
		deployArgs:       []string{},
		proxyEnvVars:     make(map[string]string),
		additionalValues: make(map[string]string),
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

	// LoadBalancer is used by default for historical and convenience reasons.
	switch b.proxyServiceType { //nolint:exhaustive
	case "":
		b.proxyServiceType = corev1.ServiceTypeLoadBalancer
	case corev1.ServiceTypeNodePort:
		// If the proxy service type is NodePort, then set the default proxy NodePort.
		if b.httpNodePort == 0 {
			b.httpNodePort = DefaultProxyNodePort
		}
	}

	return &Addon{
		logger: b.logger,

		namespace:    b.namespace,
		name:         b.name,
		deployArgs:   b.deployArgs,
		chartVersion: b.chartVersion,

		ingressControllerDisabled: b.ingressControllerDisabled,
		ingressControllerImage:    b.ingressControllerImage,
		ingressControllerImageTag: b.ingressControllerImageTag,

		proxyAdminServiceTypeLoadBalancer: b.proxyAdminServiceTypeLoadBalancer,
		proxyDBMode:                       b.proxyDBMode,
		proxyImage:                        b.proxyImage,
		proxyImageTag:                     b.proxyImageTag,
		proxyPullSecret:                   b.proxyPullSecret,
		proxyLogLevel:                     b.proxyLogLevel,
		proxyServiceType:                  b.proxyServiceType,
		proxyEnvVars:                      b.proxyEnvVars,
		proxyReadinessProbePath:           b.proxyReadinessProbePath,

		proxyEnterpriseEnabled:            b.proxyEnterpriseEnabled,
		proxyEnterpriseLicenseJSON:        b.proxyEnterpriseLicenseJSON,
		proxyEnterpriseSuperAdminPassword: b.proxyEnterpriseSuperAdminPassword,

		httpNodePort:  b.httpNodePort,
		adminNodePort: b.adminNodePort,

		additionalValues: b.additionalValues,
	}
}

func (b *Builder) WithNamespace(namespace string) *Builder {
	b.namespace = namespace
	return b
}

func (b *Builder) WithProxyImagePullSecret(server, username, password, email string) *Builder {
	b.proxyPullSecret = pullSecret{
		Server:   server,
		Username: username,
		Password: password,
		Email:    email,
	}
	return b
}

// -----------------------------------------------------------------------------
// Kong Proxy Configuration Options
// -----------------------------------------------------------------------------

// WithControllerDisabled configures the Addon in proxy only mode (bring your own control plane).
func (b *Builder) WithControllerDisabled() *Builder {
	b.ingressControllerDisabled = true

	// The default readiness probe path for the proxy is /status/ready which would make the proxy
	// with no configuration never get ready if the controller is disabled.
	b.proxyReadinessProbePath = "/status"

	return b
}

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

// WithLogLevel sets the proxy log level
func (b *Builder) WithLogLevel(level string) *Builder {
	b.proxyLogLevel = level
	return b
}

// WithProxyServiceType indicates which Service type to use for ingress traffic,
// including tcp proxy and udp proxy services.
// The default type is LoadBalancer.
func (b *Builder) WithProxyServiceType(serviceType corev1.ServiceType) *Builder {
	b.proxyServiceType = serviceType
	return b
}

// WithProxyEnvVar sets an arbitrary proxy/Kong container environment variable to a string value. The name must be
// the lowercase kong.conf style with no KONG_ prefix.
func (b *Builder) WithProxyEnvVar(name, value string) *Builder {
	b.proxyEnvVars[name] = value
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

// WithHelmChartVersion sets the helm chart version to use for the Kong proxy.
func (b *Builder) WithHelmChartVersion(version string) *Builder {
	b.chartVersion = version
	return b
}

// WithProxyReadinessProbePath sets the path to use for the proxy readiness probe.
func (b *Builder) WithProxyReadinessProbePath(path string) *Builder {
	b.proxyReadinessProbePath = path
	return b
}

// WithAdditionalValue sets arbitrary value of installing by helm.
func (b *Builder) WithAdditionalValue(name, value string) *Builder {
	b.additionalValues[name] = value
	return b
}

// WithHTTPNodePort sets the HTTP Nodeport.
func (b *Builder) WithHTTPNodePort(port int) *Builder {
	b.httpNodePort = port
	return b
}

// WithAdminNodePort sets the HTTP Nodeport.
func (b *Builder) WithAdminNodePort(port int) *Builder {
	b.adminNodePort = port
	return b
}
