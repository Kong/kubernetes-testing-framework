package kind

import (
	"fmt"
	"time"
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
)

const (
	// time to wait between GET requests
	serviceInformerTickTime = time.Millisecond * 200
)
