package kind

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kong/kubernetes-testing-framework/pkg/generators/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Consts & Vars
// -----------------------------------------------------------------------------

const (
	// ProxyDeploymentNamespace is the default namespace where the Kong proxy is expected to be deployed
	ProxyDeploymentNamespace = "kong-system"

	// ProxyDeploymentName is the default name of the Kong proxy deployment
	ProxyDeploymentName = "ingress-controller-kong"

	// proxyInformerResyncPeriod is the default time.Duration between resyncs of client-go informers
	proxyInformerResyncPeriod = time.Minute * 3

	// proxyRequestTimeout indicates the default max time we'll wait for a deployed Kong proxy to start responding to HTTP requests.
	proxyRequestTimeout = time.Minute * 3

	// time to wait between GET requests
	serviceInformerTickTime = time.Millisecond * 200
)

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Events
// -----------------------------------------------------------------------------

// ProxyReadinessEvent indicates the result of exposing the Kong proxy service in a Cluster
type ProxyReadinessEvent struct {
	URL *url.URL
	Err error
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Informers
// -----------------------------------------------------------------------------

// startProxyInformer provides a channel indicates when the proxy server is fully functional and accessible
// by providing the *url.URL by which to access it. The channel will produce a nil value on failure.
func (c *kongProxyCluster) startProxyInformer(ctx context.Context, timeout time.Time) (ready chan ProxyReadinessEvent) {
	ready = make(chan ProxyReadinessEvent)

	// we need to wait for the Kong proxy deployment
	deployment := c.startDeploymentInformer(ctx)

	go func() {
		select {
		case d := <-deployment:
			go c.startServiceInformer(ctx, d, ready, timeout)
		case <-ctx.Done():
			err := ctx.Err()
			if err == nil {
				err = fmt.Errorf("context was done before deployment received")
			}
		}
	}()

	return
}

// startDeploymentInformer will watch for the standard Kong proxy Deployment to be posted to the API and will report
// that deployment to a channel when it is found.
func (c *kongProxyCluster) startDeploymentInformer(ctx context.Context) (deployment chan *appsv1.Deployment) {
	deployment = make(chan *appsv1.Deployment)
	factory := kubeinformers.NewSharedInformerFactory(c.client, proxyInformerResyncPeriod)
	informer := factory.Apps().V1().Deployments().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d, ok := obj.(*appsv1.Deployment)
			if !ok {
				return
			}
			if d.Namespace == ProxyDeploymentNamespace && d.Name == ProxyDeploymentName {
				deployment <- d
			}
			return
		},
	})

	go func() {
		informer.Run(ctx.Done())
		close(deployment)
	}()

	return
}

// startServiceInformer exposes the provided deployment as LoadBalancer type and reports when the service has been provisioned a load balancer IP/Host
// TODO: this is a bit of a hack right now, we need to make this more idiomatic and sturdy: https://github.com/Kong/kubernetes-testing-framework/issues/14
func (c *kongProxyCluster) startServiceInformer(ctx context.Context, d *appsv1.Deployment, ready chan ProxyReadinessEvent, timeout time.Time) {
	defer close(ready)

	// expose the deployment first, if this fails then all fails
	if _, err := c.exposeProxyDeployment(ctx, d); err != nil {
		ready <- ProxyReadinessEvent{Err: err}
		return
	}

	// wait for the IP to be provisioned for the LB Service
	var u *url.URL
	var errs error
	serviceLoadBalancerReady := false
	for timeout.After(time.Now()) {
		service, err := c.Client().CoreV1().Services(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
		if err != nil {
			errs = fmt.Errorf("failures while waiting for loadbalancer service to provisioner: %w", err)
			time.Sleep(serviceInformerTickTime)
			continue
		}

		// verify if the IP for the LB has been provisioned yet
		ing := service.Status.LoadBalancer.Ingress
		if len(ing) > 0 && ing[0].IP != "" {
			// get the URL for the LB
			u, err = urlForLoadBalancerIngress(service, &ing[0])
			if err != nil {
				ready <- ProxyReadinessEvent{Err: err}
				return
			}

			// if docker network/metallb test HTTP access using the URL
			if c.enabledMetal {
				proxyReady, err := waitForKongProxy(u)
				if !proxyReady {
					ready <- ProxyReadinessEvent{Err: err}
					return
				}
			}

			if u == nil {
				errs = fmt.Errorf("url returned was nil without error: %w", errs)
			}

			serviceLoadBalancerReady = true
			break
		}
	}

	if errs != nil {
		ready <- ProxyReadinessEvent{Err: errs}
		return
	}

	if serviceLoadBalancerReady {
		ready <- ProxyReadinessEvent{URL: u}
		return
	}

	ready <- ProxyReadinessEvent{Err: fmt.Errorf("load balancer service for deployment %s/%s not ready after %s", d.Namespace, d.Name, timeout)}
}

func waitForKongProxy(u *url.URL) (proxyReady bool, err error) {
	timeout := time.Now().Add(proxyRequestTimeout)
	for timeout.After(time.Now()) {
		var resp *http.Response
		httpc := http.Client{Timeout: proxyRequestTimeout}
		resp, err = httpc.Get(u.String())
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusNotFound {
			err = fmt.Errorf("expected %d from proxy received: %s", http.StatusNotFound, resp.Status)
			continue
		}
		proxyReady = true
		return
	}
	return
}

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Private Helpers
// -----------------------------------------------------------------------------

func (c *kongProxyCluster) exposeProxyDeployment(ctx context.Context, d *appsv1.Deployment) (*corev1.Service, error) {
	svc := k8s.NewServiceForDeployment(d, corev1.ServiceTypeLoadBalancer)
	return c.client.CoreV1().Services(d.Namespace).Create(ctx, svc, metav1.CreateOptions{})
}

func urlForLoadBalancerIngress(svc *corev1.Service, ing *corev1.LoadBalancerIngress) (*url.URL, error) {
	for _, port := range svc.Spec.Ports {
		if port.Name == "proxy" {
			return url.Parse(fmt.Sprintf("http://%s:%d", ing.IP, port.Port))
		}
	}
	return nil, fmt.Errorf("no valid URL found for service %s", svc.Name)
}
