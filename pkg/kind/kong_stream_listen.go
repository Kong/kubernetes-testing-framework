package kind

import (
	"context"
	"fmt"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kong/kubernetes-testing-framework/pkg/k8s"
)

// -----------------------------------------------------------------------------
// Timeouts
// -----------------------------------------------------------------------------

// FIXME: in the future we'll use context timeouts instead see: https://github.com/Kong/kubernetes-testing-framework/issues/14

const (
	// default timeout tick interval
	waitTick = time.Second * 1

	// default amount of time to wait for changes to the Kong proxy deployment
	proxyUpdateWait = time.Minute * 7

	// default amount of time to wait for a service to be provisioned an IP by MetalLB
	serviceWait = time.Minute * 5

	// default amount of time to wait for a UDPIngress resource to be provisioned
	udpWait = time.Minute * 5

	// default amount of time to wait for an Ingress resource to be provisioned
	ingressWait = time.Minute * 7
)

// -----------------------------------------------------------------------------
// Kong Stream Listeners - Helper Functions
// -----------------------------------------------------------------------------

// UpdateProxyStreamListeners allows you to override the KONG_STREAM_LISTEN environment variable in the Kong proxy and also provide the containerports that correspond with that change.
// This function will update the ENV and the container ports of the Proxy deployment and then expose the provided ports via a LoadBalancer Service over a MetalLB provisioned IP.
// This Service will be returned by the function once the IP is provisioned and the Proxy Admin API is responding to HTTP requests successfully (so this function can take some time).
// In the function return it will provide a cleanup function that can be used to restore the proxy deployment to the state it was before changes were made once the caller has finished.
//
// FIXME: fundamentally this is a hack, before KIC 2.0 this should be replaced by functionality that ensures the status of a UDPIngress or TCPIngress resources reflects the readiness
//        of the proxy and the configuration thereof in the Admin API, the follow up for this is: https://github.com/Kong/kubernetes-ingress-controller/issues/1153
func UpdateProxyStreamListeners(ctx context.Context, cluster Cluster, name, kongStreamListen string, containerPorts ...corev1.ContainerPort) (svc *corev1.Service, cleanup func() error, err error) {
	// gather the proxy deployment and the proxy container as it will need to be specially configured to serve TCP
	proxy, err := cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Get(ctx, ProxyDeploymentName, metav1.GetOptions{})
	if err != nil {
		return
	}
	if count := len(proxy.Spec.Template.Spec.Containers); count != 1 { // we only expect a single proxy container
		err = fmt.Errorf("expected 1 container for proxy deployment, found %d", count)
		return
	}
	container := proxy.Spec.Template.Spec.Containers[0].DeepCopy()

	// override the KONG_STREAM_LISTEN env var in the proxy container environment variables
	originalVal, err := k8s.OverrideEnvVar(container, "KONG_STREAM_LISTEN", kongStreamListen)
	if err != nil {
		return
	}

	// make sure we clean up after ourselves
	cleanup = func() error {
		// remove any created Service for the proxy deployment
		if err := cluster.Client().CoreV1().Services(ProxyDeploymentNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) { // if the service is not found, that's not a problem nothing to do.
				return err
			}
		}

		// retrieve the current proxy deployment
		proxy, err := cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
		if err != nil {
			return err
		}

		// update the KONG_STREAM_LISTEN environment configuration back to its previous value
		container := proxy.Spec.Template.Spec.Containers[0].DeepCopy()
		_, err = k8s.OverrideEnvVar(container, "KONG_STREAM_LISTEN", originalVal.Value)
		if err != nil {
			return err
		}

		// remove the container ports that were added
		newPorts := make([]corev1.ContainerPort, 0, len(container.Ports))
		for _, port := range container.Ports {
			includePort := true
			for _, configuredPort := range containerPorts {
				if port.Name == configuredPort.Name {
					includePort = false
					break
				}
			}

			if includePort {
				newPorts = append(newPorts, port)
			}
		}
		container.Ports = newPorts

		// revert the corev1.Container to its state prior to the test
		proxy.Spec.Template.Spec.Containers[0] = *container
		_, err = cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Update(ctx, proxy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		// ensure that the proxy deployment is ready before we proceed
		ready := false
		timeout := time.Now().Add(proxyUpdateWait)
		for timeout.After(time.Now()) {
			d, err := cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Get(ctx, proxy.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if d.Status.ReadyReplicas == d.Status.Replicas && d.Status.AvailableReplicas == d.Status.Replicas && d.Status.UnavailableReplicas < 1 {
				ready = true
				break
			}

			time.Sleep(waitTick)
		}

		if ready {
			return nil
		}

		return fmt.Errorf("proxy did not become ready after %s", proxyUpdateWait)
	}

	// add the provided container ports to the pod and update the deployment with the new configurations
	container.Ports = append(container.Ports, containerPorts...)
	proxy.Spec.Template.Spec.Containers[0] = *container
	proxy, err = cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Update(ctx, proxy, metav1.UpdateOptions{})
	if err != nil {
		return
	}

	// ensure that the proxy deployment is ready before we proceed
	ready := false
	timeout := time.Now().Add(proxyUpdateWait)
	for timeout.After(time.Now()) {
		var d *appsv1.Deployment
		d, err = cluster.Client().AppsV1().Deployments(ProxyDeploymentNamespace).Get(ctx, proxy.Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		// the deployment itself needs to be ready
		if d.Status.ReadyReplicas == d.Status.Replicas && d.Status.AvailableReplicas == d.Status.Replicas && d.Status.UnavailableReplicas < 1 {
			var proxySVC *corev1.Service
			proxySVC, err = cluster.Client().CoreV1().Services(proxy.Namespace).Get(ctx, proxy.Name, metav1.GetOptions{})
			if err != nil {
				return
			}
			if len(proxySVC.Status.LoadBalancer.Ingress) == 1 {
				// once an ingress IP is provisioned, we check to make sure we can get a 200 OK from the admin API as well
				ip := proxySVC.Status.LoadBalancer.Ingress[0].IP
				resp, err := http.Get(fmt.Sprintf("http://%s:8001/services", ip))
				if err != nil {
					continue
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					ready = true
					break
				}
			}
		}
	}

	// if the proxy deployment is ready, expose the new container ports via a LoadBalancer Service
	if ready {
		// configure the provided container ports for a LB service
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeLoadBalancer,
				Selector: proxy.Spec.Selector.MatchLabels,
			},
		}
		for _, containerPort := range containerPorts {
			servicePort := corev1.ServicePort{
				Protocol:   containerPort.Protocol,
				Port:       containerPort.ContainerPort,
				TargetPort: intstr.FromInt(int(containerPort.ContainerPort)),
			}
			svc.Spec.Ports = append(svc.Spec.Ports, servicePort)
		}
		svc, err = cluster.Client().CoreV1().Services(ProxyDeploymentNamespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return
		}

		// wait for the LB service to be provisioned
		provisioned := false
		timeout := time.Now().Add(serviceWait)
		for timeout.After(time.Now()) {
			svc, err = cluster.Client().CoreV1().Services(ProxyDeploymentNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				ing := svc.Status.LoadBalancer.Ingress[0]
				if ip := ing.IP; ip != "" {
					provisioned = true
					break
				}
			}
			time.Sleep(waitTick)
		}
		if !provisioned {
			err = fmt.Errorf("load balancer service for deployment %s did not provision successfully within %s", name, serviceWait)
		}
	} else { // if the deployment didn't become ready within the timeout period, bail out there's nothing more we can do.
		err = fmt.Errorf("deployment not ready after %s", timeout)
	}

	// FIXME: there appears to be a race condition that can occur in kong upstream if the Admin API is restarted and then updated
	//        too quickly. Effectively the API will return 200 OK but it's not actually ready to accept updates. Given a few seconds
	//        this will settle so as a workaround we're sleeping here, but we need to dig deeper into this problem to actually fix it.
	//        NOTE: 10 seconds was chosen because it tested positively over several runs locally in Linux, and in Github Actions.
	time.Sleep(time.Second * 10)

	return
}
