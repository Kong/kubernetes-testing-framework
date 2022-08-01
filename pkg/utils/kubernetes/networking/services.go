package networking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

// WaitForTCPAddressConnected waits for a tcp address to be connected.
// if the TCP address is successfully connected from the pod inside cluster, it returns nil
// returns error if the address could not be connected with TCP within duration `timeout`.
func WaitForTCPAddressConnected(ctx context.Context, c *kubernetes.Clientset, jobNamespace string, address string, timeout time.Duration) error {
	// TODO: push the image to kong repo and put the tcpTestImage to a global var/const
	var tcpTestImage = "richardyi/ktf-tcptest:0.0" // should FIX this, currently my personal repo, but available.

	id := uuid.NewString()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tcp-test-" + id,
			Labels: map[string]string{
				"app":  "tcp-test",
				"task": id,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "tcp-test",
							Image: tcpTestImage,
							Args: []string{
								"-test-address=" + address,
								"-test-timeout=" + timeout.String(),
							},
						},
					},
				},
			},
		},
	}
	// TODO: use another namespace.
	job, err := c.BatchV1().Jobs(jobNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	defer func() {
		// added nolint here because we do not care about error in deleting the job, only the job status.
		c.BatchV1().Jobs(jobNamespace).Delete(ctx, job.Name, metav1.DeleteOptions{}) // nolint:errcheck
	}()

	jobWatch, err := c.BatchV1().Jobs(job.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "task=" + id,
	})
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context deadline exceeded")
		case event := <-jobWatch.ResultChan():
			if event.Type == watch.Modified {
				obj := event.Object
				observedJob, ok := obj.(*batchv1.Job)
				if !ok {
					return fmt.Errorf("wrong type of object in watch: %T", obj)
				}
				if observedJob.Name != job.Name {
					continue
				}
				if observedJob.Status.Succeeded > 0 {
					return nil
				}
				if observedJob.Status.Failed > 0 {
					return fmt.Errorf("job failed")
				}
			}
		}
	}
}

// WaitForServicePortConnected wait for a port of a service be able to be connected with TCP.
// if useDNS is true, it tries the address name.namespace.svc:port.
// else it uses clusterIP:port.
func WaitForServicePortConnected(ctx context.Context, c *kubernetes.Clientset, namespace string, name string, port int, useDNS bool, timeout time.Duration) error {
	service, err := c.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if useDNS {
		address := fmt.Sprintf("%s.%s.svc:%d", name, namespace, port)
		return WaitForTCPAddressConnected(ctx, c, namespace, address, timeout)
	}

	clusterIP := service.Spec.ClusterIP
	address := fmt.Sprintf("%s:%d", clusterIP, port)
	return WaitForTCPAddressConnected(ctx, c, namespace, address, timeout)
}
