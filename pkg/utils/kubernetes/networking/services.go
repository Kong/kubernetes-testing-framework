package networking

import (
	"context"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WaitForServiceLoadBalancerAddress waits for a service provided by
// namespace/name to have an ingress IP or Host provisioned and returns that
// address. This function will throw an error if the service gets provisioned
// more than a single address, that is not supported. The context provided
// should have a timeout associated with it or you're going to have a bad time.
func WaitForServiceLoadBalancerAddress(ctx context.Context, c *kubernetes.Clientset, namespace, name string) (string, bool, error) {
	for {
		select {
		case <-ctx.Done():
			return "", false, fmt.Errorf("context completed while waiting for loadbalancer service to provision: %w", ctx.Err())
		default:
			// retrieve a fresh copy of the service
			service, err := c.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return "", false, fmt.Errorf("error while trying to retrieve registry service: %w", err)
			}
			lbing := service.Status.LoadBalancer.Ingress

			// don't support services which have multiple addresses
			if len(lbing) > 1 {
				return "", false, fmt.Errorf("services with more than one load balancer address are not supported (found %d)", len(lbing))
			}

			// verify whether the loadbalancer details are provisioned
			if len(lbing) == 1 {
				for _, ing := range lbing {
					if ing.Hostname != "" {
						return ing.Hostname, false, nil
					}
					if ing.IP != "" {
						return ing.IP, true, nil
					}
				}
			}
		}
	}
}

// WaitForConnectionOnServicePort waits until it can make successful TCP connections
// to a service (provided by namespace/name). This will temporarily create a LoadBalancer
// type Service to allow connections to the Service and port from outside the cluster while
// the connection attempts are made using the LoadBalancer public address.
func WaitForConnectionOnServicePort(ctx context.Context, c *kubernetes.Clientset, namespace, name string, port int, dialTimeout time.Duration) error {
	service, err := c.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	lbServiceName := "templb-" + name
	tempLoadBalancer := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      lbServiceName,
			Labels: map[string]string{
				"corresponding-service": name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			// copy the selector and ports of the service to check.
			Selector: service.Spec.Selector,
			Ports:    service.Spec.Ports,
		},
	}

	// empty selector, we should create the endpoints separately.
	// if the target service does not have a selector, it usually means that
	// the endpoints of the target server is manually created, but not chosen from pods by labels in selector.
	// so we need to manually create the same endpoints as the target service has here.
	if service.Spec.Selector == nil || len(service.Spec.Selector) == 0 {
		endpoints, err := c.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// copy the endpoints for the lb service, and set metadata.
		tempEndpoints := endpoints.DeepCopy()
		tempEndpoints.ObjectMeta = metav1.ObjectMeta{
			Namespace: namespace,
			Name:      lbServiceName,
			Labels: map[string]string{
				"corresponding-service": name,
			},
		}
		_, err = c.CoreV1().Endpoints(namespace).Create(ctx, tempEndpoints, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		defer func() {
			err := c.CoreV1().Endpoints(namespace).Delete(ctx, lbServiceName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				fmt.Printf("failed to delete endpoints %s/%s after testing, error %v\n",
					namespace, lbServiceName, err)
			}
		}()
	}

	_, err = c.CoreV1().Services(namespace).Create(ctx, tempLoadBalancer, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	defer func() {
		err := c.CoreV1().Services(namespace).Delete(ctx, lbServiceName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			fmt.Printf("failed to delete service %s/%s after testing, error %v\n",
				namespace, lbServiceName, err)
		}
	}()

	ip, _, err := WaitForServiceLoadBalancerAddress(ctx, c, namespace, lbServiceName)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Second)
	address := fmt.Sprintf("%s:%d", ip, port)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while waiting for %s:%d to be connected", ip, port)
		case <-ticker.C:
			dialer := &net.Dialer{Timeout: dialTimeout}
			_, err := dialer.Dial("tcp", address)
			if err == nil {
				return nil
			}
		}
	}
}
