package utils

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

const (
	// AdminNamespace is the namespace used for administrative acts
	// by KTF for purposes such as deploying addons.
	AdminNamespace = "ktf-system"

	// AdminBinding is the name of the ClusterRoleBinding created to
	// bind the AdminNamespace's default service account to the
	// "cluster-admin" role.
	AdminBinding = "ktf-admin"
)

// ClusterInitHooks are generic hooks to provision resources on a test cluster
// regardless of which cluster type (e.g. kind, gke). This includes the creation
// of some special administrative namespaces and service accounts.
func ClusterInitHooks(ctx context.Context, cluster clusters.Cluster) error {
	const (
		serviceAccountWaitTime = time.Second
	)
	// create the admin namespace if it doesn't already exist
	namespace, err := cluster.Client().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AdminNamespace}}, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) { // tolerate the namespace already existing
			return err
		}
	}

	// wait for the default service account to be available
	var defaultSAFound bool
	var defaultSA *corev1.ServiceAccount

	for !defaultSAFound {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed before cluster init hooks could finish: %w", ctx.Err())
		default:
			defaultSA, err = cluster.Client().CoreV1().ServiceAccounts(namespace.Name).Get(ctx, "default", metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					select {
					case <-time.After(serviceAccountWaitTime):
						continue // try again if its not there yet
					case <-ctx.Done():
						return fmt.Errorf("context completed before cluster init hooks could finish: %w", ctx.Err())
					}
				}
				return err // don't tolerate any errors except 404
			}
			defaultSAFound = true
		}
	}

	// give the default SA in this namespace cluster admin
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: AdminBinding,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      defaultSA.Name,
			Namespace: defaultSA.Namespace,
		}},
	}
	_, err = cluster.Client().RbacV1().ClusterRoleBindings().Create(ctx, &crb, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) { // tolerate the crb already existing
			return err
		}
	}

	return nil
}
