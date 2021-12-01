package networking

import (
	"context"
	"fmt"

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
