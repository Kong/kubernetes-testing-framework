package clusters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeployIngress is a helper and function to deploy an Ingress object to a cluster handling
// the version of the Ingress object for the caller so they don't have to.
// TODO: once we stop supporting old Kubernetes versions <1.19 we can remove this.
func DeployIngress(ctx context.Context, c Cluster, namespace string, ingress runtime.Object) (err error) {
	switch obj := ingress.(type) {
	case *netv1.Ingress:
		_, err = c.Client().NetworkingV1().Ingresses(namespace).Create(ctx, obj, metav1.CreateOptions{})
	case *netv1beta1.Ingress:
		_, err = c.Client().NetworkingV1beta1().Ingresses(namespace).Create(ctx, obj, metav1.CreateOptions{})
	case *extv1beta1.Ingress:
		_, err = c.Client().ExtensionsV1beta1().Ingresses(namespace).Create(ctx, obj, metav1.CreateOptions{})
	default:
		err = fmt.Errorf("%T is not a supported ingress type", ingress)
	}
	return
}

// DeleteIngress is a helper and function to delete an Ingress object to a cluster handling
// the version of the Ingress object for the caller so they don't have to.
// TODO: once we stop supporting old Kubernetes versions <1.19 we can remove this.
func DeleteIngress(ctx context.Context, c Cluster, namespace string, ingress runtime.Object) (err error) {
	switch obj := ingress.(type) {
	case *netv1.Ingress:
		err = c.Client().NetworkingV1().Ingresses(namespace).Delete(ctx, obj.Name, metav1.DeleteOptions{})
	case *netv1beta1.Ingress:
		err = c.Client().NetworkingV1beta1().Ingresses(namespace).Delete(ctx, obj.Name, metav1.DeleteOptions{})
	case *extv1beta1.Ingress:
		err = c.Client().ExtensionsV1beta1().Ingresses(namespace).Delete(ctx, obj.Name, metav1.DeleteOptions{})
	default:
		err = fmt.Errorf("%T is not a supported ingress type", ingress)
	}
	return
}

// GetIngressLoadbalancerStatus is a partner to the above DeployIngress function which will
// given an Ingress object provided by the caller determine the version and pull a fresh copy
// of the current LoadBalancerStatus for that Ingress object without the caller needing to be
// aware of which version of Ingress they're using.
// TODO: once we stop supporting old Kubernetes versions <1.19 we can remove this.
func GetIngressLoadbalancerStatus(ctx context.Context, c Cluster, namespace string, ingress runtime.Object) (*corev1.LoadBalancerStatus, error) {
	switch obj := ingress.(type) {
	case *netv1.Ingress:
		refresh, err := c.Client().NetworkingV1().Ingresses(namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &refresh.Status.LoadBalancer, nil
	case *netv1beta1.Ingress:
		refresh, err := c.Client().NetworkingV1beta1().Ingresses(namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &refresh.Status.LoadBalancer, nil
	case *extv1beta1.Ingress:
		refresh, err := c.Client().ExtensionsV1beta1().Ingresses(namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &refresh.Status.LoadBalancer, nil
	default:
		return nil, fmt.Errorf("%T is not a supported ingress type", ingress)
	}
}

// CreateNamespace create customized namespace
func CreateNamespace(ctx context.Context, cluster Cluster, namespace string) error {
	nsName := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	fmt.Printf("creating namespace %s.", namespace)
	_, err := cluster.Client().CoreV1().Namespaces().Create(context.Background(), nsName, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed creating namespace %s, err %v", namespace, err)
	}

	nsList, err := cluster.Client().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed creating namespace %v", err)
	}
	for _, item := range nsList.Items {
		if item.Name == namespace && item.Status.Phase == corev1.NamespaceActive {
			fmt.Printf("created namespace %s successfully.", namespace)
			return nil
		}
	}

	return fmt.Errorf("failed creating namespace %s", namespace)
}
