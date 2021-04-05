package kind

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Consts & Vars
// -----------------------------------------------------------------------------

var (
	// ProxyAdminPort is the port on the service at which the Kong Admin API can be reached by default.
	ProxyAdminPort = 8001

	// ProxyPort is the port on the service at which the Kong proxy can be reached by default.
	ProxyPort = 80

	// ProxyNamespace is the default namespace where the Kong proxy is expected to be deployed
	ProxyNamespace = "kong-system"

	// ProxyDeploymentName is the default name of the Kong proxy deployment
	ProxyDeploymentName = "ingress-controller-kong"

	// ProxyAdminServiceName indicates the name of the Service that's serving the Admin API
	ProxyAdminServiceName = fmt.Sprintf("%s-admin", ProxyDeploymentName)

	// ProxyServiceName indicates the name of the Service that's serving the Proxy
	ProxyServiceName = fmt.Sprintf("%s-proxy", ProxyDeploymentName)

	// ProxyUDPServiceName provides the name of the LoadBalancer service the proxy uses for UDP traffic.
	// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
	//       See: https://github.com/Kong/charts/issues/329
	ProxyUDPServiceName = fmt.Sprintf("%s-udp", ProxyDeploymentName)
)
