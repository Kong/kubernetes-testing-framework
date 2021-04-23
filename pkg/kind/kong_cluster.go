package kind

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kong/kubernetes-testing-framework/pkg/helm"
	"github.com/kong/kubernetes-testing-framework/pkg/metallb"
)

// -----------------------------------------------------------------------------
// ClusterConfiguration - Public Types
// -----------------------------------------------------------------------------

const (
	// EnvKeepCluster is the environment variable that can be set to "true" in order
	// to circumvent teardown during cleanup of clusters in order to allow a user to inspect them instead.
	EnvKeepCluster = "KIND_KEEP_CLUSTER"
)

// ClusterConfigurationWithKongProxy is an object representing a Kind cluster's configuration and can effectively
// be used as a factory for kind cluster deployments. Clusters created from these configurations are
// opinionated, and will always automatically pre-deploy a Kong proxy service.
type ClusterConfigurationWithKongProxy struct {
	// DockerNetwork indicates the name of the Docker network to use for LoadBalancer IPs
	DockerNetwork string

	// EnableMetalLB instructions the deployment of MetalLB to support provisioning LoadBalancer Services in the cluster.
	EnableMetalLB bool
}

// -----------------------------------------------------------------------------
// ClusterConfiguration - Factory Methods
// -----------------------------------------------------------------------------

// Deploy is a factory method to generate kind.Cluster objects given the configuration, with new names being selected on each deploy.
func (c *ClusterConfigurationWithKongProxy) Deploy(ctx context.Context) (Cluster, chan ProxyReadinessEvent, error) {
	return c.DeployWithName(ctx, uuid.New().String())
}

// DeployWithName is a factory method to generate kind.Cluster objects given the configuration with a custom name provided.
func (c *ClusterConfigurationWithKongProxy) DeployWithName(ctx context.Context, name string) (Cluster, chan ProxyReadinessEvent, error) {

	if c.DockerNetwork == "" {
		c.DockerNetwork = DefaultKindDockerNetwork
	}

	err := CreateCluster(name)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateCluster() failed: %w", err)
	}

	cfg, kc, err := ClientForCluster(name)
	if err != nil {
		return nil, nil, err
	}

	cluster := &kongProxyCluster{
		name:         name,
		client:       kc,
		cfg:          cfg,
		enabledMetal: c.EnableMetalLB,
	}

	if cluster.enabledMetal {
		if err := metallb.DeployMetallbForKindCluster(kc, name, c.DockerNetwork); err != nil {
			return nil, nil, err
		}
	}

	if err := helm.DeployKongProxyOnly(name); err != nil {
		return nil, nil, err
	}

	// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
	//       See: https://github.com/Kong/charts/issues/329
	if err := runUDPServiceHack(ctx, cluster); err != nil {
		return nil, nil, err
	}

	ready := make(chan ProxyReadinessEvent)
	go cluster.ProxyReadinessInformer(ctx, ready)

	return cluster, ready, nil
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Interface Implementation
// -----------------------------------------------------------------------------

func (c *kongProxyCluster) Name() string {
	return c.name
}

func (c *kongProxyCluster) Cleanup() error {
	if os.Getenv(EnvKeepCluster) == "" {
		return DeleteKindCluster(c.name)
	}
	return nil
}

func (c *kongProxyCluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *kongProxyCluster) Config() *rest.Config {
	return c.cfg
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Types
// -----------------------------------------------------------------------------

type kongProxyCluster struct {
	name         string
	client       *kubernetes.Clientset
	cfg          *rest.Config
	enabledMetal bool
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Functions
// -----------------------------------------------------------------------------

// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
//       See: https://github.com/Kong/charts/issues/329
func runUDPServiceHack(ctx context.Context, cluster Cluster) error {
	udpServicePorts := []corev1.ServicePort{{
		Name:     ProxyUDPServiceName,
		Protocol: corev1.ProtocolUDP,
		Port:     9999,
	}}
	udpService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: ProxyUDPServiceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app.kubernetes.io/component": "app",
				"app.kubernetes.io/instance":  "ingress-controller",
				"app.kubernetes.io/name":      "kong",
			},
			Ports: udpServicePorts,
		},
	}
	_, err := cluster.Client().CoreV1().Services(ProxyNamespace).Create(ctx, udpService, metav1.CreateOptions{})
	return err
}
