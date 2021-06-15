package kind

import (
	"fmt"
	"time"
)

// -----------------------------------------------------------------------------
// Public Consts & Vars
// -----------------------------------------------------------------------------

const (
	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "KIND_KEEP_CLUSTER"

	// DefaultKindDockerNetwork is the Docker network that a kind cluster uses by default.
	DefaultKindDockerNetwork = "kind"
)

var (
	// ProxyReadinessWaitTick is the amount of time to wait between status checks for a Kind cluster.
	ProxyReadinessWaitTick = time.Millisecond * 200

	// ProxyAdminPort is the port on the service at which the Kong Admin API can be reached by default.
	ProxyAdminPort = 8001

	// ProxyPort is the port on the service at which the Kong proxy can be reached by default.
	ProxyPort = 80

	// ProxyHTTPSPort is the port on the service at which the Kong proxy can be reached by default.
	ProxyHTTPSPort = 443

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
