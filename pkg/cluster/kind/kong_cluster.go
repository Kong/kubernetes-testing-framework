package kind

import (
	"context"
	"fmt"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
	ktfcluster "github.com/kong/kubernetes-testing-framework/pkg/cluster"
)

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Interface Implementation
// -----------------------------------------------------------------------------

func (c *kongProxyCluster) Name() string {
	return c.name
}

func (c *kongProxyCluster) Type() cluster.Type {
	return KindClusterType
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

func (c *kongProxyCluster) Addons() []cluster.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]cluster.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *kongProxyCluster) DeployAddon(addon cluster.Addon) error {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.addons[addon.Name()]; ok {
		return fmt.Errorf("addon component %s is already loaded into cluster %s", addon.Name(), c.Name())
	}

	if err := addon.Deploy(c); err != nil {
		return err
	}

	c.addons[addon.Name()] = addon

	return nil
}

func (c *kongProxyCluster) DeleteAddon(addon cluster.Addon) error {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.addons[addon.Name()]; !ok {
		return nil
	}

	if err := addon.Delete(c); err != nil {
		return err
	}

	delete(c.addons, addon.Name())

	return nil
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Types
// -----------------------------------------------------------------------------

type kongProxyCluster struct {
	name   string
	client *kubernetes.Clientset
	cfg    *rest.Config
	addons map[string]cluster.Addon
	l      *sync.RWMutex
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Functions
// -----------------------------------------------------------------------------

// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
//       See: https://github.com/Kong/charts/issues/329
func runUDPServiceHack(ctx context.Context, cluster ktfcluster.Cluster) error {
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
