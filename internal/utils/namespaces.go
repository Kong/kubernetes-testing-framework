package utils

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// IsNamespaceAvailable checks for all Daemonsets, Deployment and Services
// in a given namespace to see if they are available (ready for minimum number
// of seconds).
//
// If the namespace is not yet available a list of the components being waited
// on will be provided.
func IsNamespaceAvailable(ctx context.Context, cluster clusters.Cluster, namespace string) (waitForObjects []runtime.Object, available bool, err error) {
	// if the namespace itself isn't available yet, not ready
	_, err = cluster.Client().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			waitForObjects = append(waitForObjects, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
			err = nil
		}
		return
	}

	// check daemonsets for availability
	var daemonsets *appsv1.DaemonSetList
	daemonsets, err = cluster.Client().AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(daemonsets.Items); i++ {
		daemonset := &(daemonsets.Items[i])
		if daemonset.Status.NumberAvailable < 1 {
			waitForObjects = append(waitForObjects, daemonset)
		}
	}

	// check deployments for availability
	var deployments *appsv1.DeploymentList
	deployments, err = cluster.Client().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(deployments.Items); i++ {
		deployment := &(deployments.Items[i])
		if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			waitForObjects = append(waitForObjects, deployment)
		}
	}

	// check services for availability
	var services *corev1.ServiceList
	services, err = cluster.Client().CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(services.Items); i++ {
		service := &(services.Items[i])
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer && len(service.Status.LoadBalancer.Ingress) < 1 {
			waitForObjects = append(waitForObjects, service)
		}
	}

	// if there are no daemonsets or deployments present we can't consider this ready yet
	// the expectation is that at least one (of any type) exists.
	if (len(daemonsets.Items) + len(deployments.Items)) == 0 {
		return
	}

	available = len(waitForObjects) == 0
	return
}
