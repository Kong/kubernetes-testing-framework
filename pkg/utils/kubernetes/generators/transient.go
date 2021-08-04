package generators

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// GenerateNamespace creates a transient testing namespace given the cluster to create
// it on and a creator ID. The namespace will be given a UUID for a name, and the creatorID
// will be applied to the TestResourceLabel for automated cleanup.
func GenerateNamespace(ctx context.Context, cluster clusters.Cluster, creatorID string) (*corev1.Namespace, error) {
	if creatorID == "" {
		return nil, fmt.Errorf(`empty string "" is not a valid creator ID`)
	}

	name := uuid.NewString()
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				TestResourceLabel: creatorID,
			},
		},
	}

	return cluster.Client().CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
}

// CleanupGeneratedResources cleans up all resources created by the given creator ID.
func CleanupGeneratedResources(ctx context.Context, cluster clusters.Cluster, creatorID string) error {
	if creatorID == "" {
		return fmt.Errorf(`empty string "" is not a valid creator ID`)
	}

	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", TestResourceLabel, creatorID),
	}

	namespaceList, err := cluster.Client().CoreV1().Namespaces().List(ctx, listOpts)
	if err != nil {
		return err
	}

	namespacesToCleanup := make(map[string]*corev1.Namespace)
	for i := 0; i < len(namespaceList.Items); i++ {
		namespace := &(namespaceList.Items[i])
		namespacesToCleanup[namespace.Name] = namespace
	}

	for len(namespacesToCleanup) > 0 {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context completed with error while waiting for cleanup: %w", err)
			}
			return fmt.Errorf("context completed while waiting for cleanup")
		default:
			for _, namespace := range namespaceList.Items {
				if err := cluster.Client().CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
					if errors.IsNotFound(err) {
						delete(namespacesToCleanup, namespace.Name)
					} else {
						return fmt.Errorf("failed to delete namespace resource %s: %w", namespace.Name, err)
					}
				}
			}
		}
	}

	return nil
}
