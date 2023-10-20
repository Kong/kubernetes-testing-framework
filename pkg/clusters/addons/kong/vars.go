package kong

// -----------------------------------------------------------------------------
// Kong Addon - Defaults
// -----------------------------------------------------------------------------

const (
	// KongHelmRepoURL indicates the URL that will be used for pulling Kong Helm charts.
	KongHelmRepoURL = "https://charts.konghq.com"

	// DefaultDBMode indicates which dbmode for the Kong proxy should be used if no other is specified.
	DefaultDBMode = "none"

	// DefaultNamespace is the default namespace where the Kong proxy is expected to be deployed
	DefaultNamespace = "kong-system"

	// DefaultDeploymentName is the default name of the Kong proxy deployment
	DefaultDeploymentName = "ingress-controller"

	// DefaultReleaseName is the Helm release name of the Kong proxy
	DefaultReleaseName = DefaultDeploymentName + "-kong"

	// DefaultAdminServiceName indicates the name of the Service that's serving the Admin API
	DefaultAdminServiceName = DefaultReleaseName + "-admin"

	// DefaultAdminServicePort is the port on the service at which the Kong Admin API can be reached by default.
	DefaultAdminServicePort = 8001

	// DefaultProxyServiceName indicates the name of the Service that's serving the Proxy
	DefaultProxyServiceName = DefaultReleaseName + "-proxy"

	// DefaultProxyTCPServicePort is the port on the service at which the Kong proxy can be reached by default.
	DefaultProxyTCPServicePort = 80

	// DefaultProxyHTTPServicePort is the port on the service at which the Kong proxy servers HTTP traffic by default.
	DefaultProxyHTTPPort = 80

	// DefaultProxyTLSServicePort is the port on the service at which the Kong proxy can be reached by default.
	DefaultProxyTLSServicePort = 443

	// DefaultUDPServiceName provides the name of the LoadBalancer service the proxy uses for UDP traffic.
	DefaultUDPServiceName = DefaultReleaseName + "-udp-proxy"

	// DefaultUDPServicePort indicates the default open port to be found on the Kong proxy's UDP service.
	DefaultUDPServicePort = 9999

	// DefaultTCPServicePort indicates the default open port that will be used for TCP traffic.
	DefaultTCPServicePort = 8888

	// DefaultTLSServicePort indicates the default open port that will be used for TLS traffic.
	DefaultTLSServicePort = 8899

	// DefaultProxyNodePort indicates the default NodePort that will be used for
	// the proxy when applicable.
	DefaultProxyNodePort = 30080

	// DefaultAdminNodePort indicates the default NodePort that will be used for
	// the admin API when applicable.
	DefaultAdminNodePort = 32080
)
