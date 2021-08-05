//+build integration_tests

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	httpbinWait = time.Minute * 2
	waitTick    = time.Second
)

func TestGenerators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("creating a test environment to test generators")
	env, err := environments.NewBuilder().Build(ctx)
	require.NoError(t, err)

	t.Log("waiting for the test environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("configuring 3 unique creators for namespace generation with different quotas")
	creator1, creator1NamespaceCount := uuid.NewString(), 5
	creator2, creator2NamespaceCount := uuid.NewString(), 2
	creator3, creator3NamespaceCount := uuid.NewString(), 1
	testingNamespaces := make(map[string][]*corev1.Namespace)

	t.Logf("creating %d namespaces for creator ID 1", creator1NamespaceCount)
	for i := 1; i < creator1NamespaceCount; i++ {
		testingNamespace, err := generators.GenerateNamespace(ctx, env.Cluster(), creator1)
		require.NoError(t, err)
		testingNamespaces[creator1] = append(testingNamespaces[creator1], testingNamespace)
	}

	t.Logf("creating %d namespaces for creator ID 2", creator2NamespaceCount)
	for i := 1; i < creator2NamespaceCount; i++ {
		testingNamespace, err := generators.GenerateNamespace(ctx, env.Cluster(), creator2)
		require.NoError(t, err)
		testingNamespaces[creator2] = append(testingNamespaces[creator2], testingNamespace)
	}

	t.Logf("creating %d namespaces for creator ID 3", creator3NamespaceCount)
	for i := 1; i < creator3NamespaceCount; i++ {
		testingNamespace, err := generators.GenerateNamespace(ctx, env.Cluster(), creator3)
		require.NoError(t, err)
		testingNamespaces[creator3] = append(testingNamespaces[creator3], testingNamespace)
	}

	t.Logf("deploying a testing pod in each namespace and verifying they all deploy properly")
	for _, namespaces := range testingNamespaces {
		for _, namespace := range namespaces {
			container := generators.NewContainer("httpbin", "kennethreitz/httpbin", 80)
			deployment := generators.NewDeploymentForContainer(container)
			_, err := env.Cluster().Client().AppsV1().Deployments(namespace.Name).Create(ctx, deployment, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		for _, namespace := range namespaces {
			require.Eventually(t, func() bool {
				deployment, err := env.Cluster().Client().AppsV1().Deployments(namespace.Name).Get(ctx, "httpbin", metav1.GetOptions{})
				require.NoError(t, err)
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			}, httpbinWait, waitTick)
		}
	}

	t.Log("performing generated resource cleanup for creator ID 3")
	require.NoError(t, generators.CleanupGeneratedResources(ctx, env.Cluster(), creator3))

	t.Log("verifying that creator ID 3's namespaces were all removed from the cluster")
	for _, namespace := range testingNamespaces[creator3] {
		_, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		require.Error(t, err)
		require.True(t, errors.IsNotFound(err))
	}

	t.Log("verifying that the other creator ID's namespaces were left alone")
	for _, namespace := range append(testingNamespaces[creator1], testingNamespaces[creator2]...) {
		namespace, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Nil(t, namespace.DeletionTimestamp)
	}

	t.Log("performing generated resource cleanup for creator ID 1")
	require.NoError(t, generators.CleanupGeneratedResources(ctx, env.Cluster(), creator1))

	t.Log("verifying that creator ID 1's namespaces were all removed from the cluster")
	for _, namespace := range testingNamespaces[creator1] {
		_, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		require.Error(t, err)
		require.True(t, errors.IsNotFound(err))
	}

	t.Log("verifying that the other creator ID's namespaces were left alone")
	for _, namespace := range testingNamespaces[creator2] {
		namespace, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Nil(t, namespace.DeletionTimestamp)
	}

	t.Log("performing generated resource cleanup for creator ID 2")
	require.NoError(t, generators.CleanupGeneratedResources(ctx, env.Cluster(), creator2))

	t.Log("verifying that creator ID 2's namespaces were all removed from the cluster")
	for _, namespace := range testingNamespaces[creator2] {
		_, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		require.Error(t, err)
		require.True(t, errors.IsNotFound(err))
	}
}
