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
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// -----------------------------------------------------------------------------
// Kong Proxy Cluster - Consts & Vars
// -----------------------------------------------------------------------------

const (
	// proxyInformerResyncPeriod is the default time.Duration between resyncs of client-go informers
	proxyInformerResyncPeriod = time.Minute * 3

	// proxyDeploymentNamespace is the default namespace where the Kong proxy is expected to be deployed
	proxyDeploymentNamespace = "kong-system"

	// proxyDeploymentName is the default name of the Kong proxy deployment
	proxyDeploymentName = "ingress-controller-kong"

	// proxyRequestTimeout indicates the default max time we'll wait for a deployed Kong proxy to start responding to HTTP requests.
	proxyRequestTimeout = time.Minute * 3
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
func (c *kongProxyCluster) startProxyInformer(ctx context.Context) (ready chan ProxyReadinessEvent) {
	ready = make(chan ProxyReadinessEvent)

	// we need to wait for the Kong proxy deployment
	deployment := c.startDeploymentInformer(ctx)

	go func() {
		select {
		case d := <-deployment:
			// start the service informer to watch updates to the service we're about to create
			go c.startServiceInformer(ctx, types.NamespacedName{Namespace: d.Namespace, Name: d.Name}, ready)

			// expose the deployment
			_, err := c.exposeProxyDeployment(ctx, d)
			if err != nil {
				ready <- ProxyReadinessEvent{Err: err}
				close(ready)
				return
			}
		case <-ctx.Done():
			err := ctx.Err()
			if err == nil {
				err = fmt.Errorf("context was done before deployment received")
			}
			ready <- ProxyReadinessEvent{Err: err}
			close(ready)
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
			if d.Namespace == proxyDeploymentNamespace && d.Name == proxyDeploymentName {
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

func (c *kongProxyCluster) startServiceInformer(ctx context.Context, nsn types.NamespacedName, ready chan ProxyReadinessEvent) {
	factory := kubeinformers.NewSharedInformerFactory(c.client, proxyInformerResyncPeriod)
	informer := factory.Core().V1().Services().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// this UpdateFunc is responsible for closing the ready channel once the proxy URL is ready (or if an error occurs)
		UpdateFunc: func(old1, new1 interface{}) {
			newService, ok := new1.(*corev1.Service)
			if !ok {
				err := fmt.Errorf("somehow got unexpected type instead of corev1.Service: %T", new1)
				ready <- ProxyReadinessEvent{Err: err}
				close(ready)
				return
			}

			// if this is the correct service, operate on it
			if newService.Namespace == nsn.Namespace && newService.Name == nsn.Name {
				// verify if the IP for the LB has been provisioned yet
				ing := newService.Status.LoadBalancer.Ingress
				if len(ing) > 0 && ing[0].IP != "" {
					// get the URL for the LB
					u, err := urlForLoadBalancerIngress(newService, &ing[0])
					if err != nil {
						ready <- ProxyReadinessEvent{Err: err}
						close(ready)
						return
					}

					// if docker network/metallb test HTTP access using the URL
					if c.enabledMetal {
						proxyReady, err := waitForKongProxy(u)
						if !proxyReady {
							ready <- ProxyReadinessEvent{Err: err}
							close(ready)
							return
						}
					}

					ready <- ProxyReadinessEvent{URL: u}
					close(ready)
				}
			}
		},
	})
	go informer.Run(ctx.Done())
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
