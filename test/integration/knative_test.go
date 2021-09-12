package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/knative"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestEnvironmentWithKnative(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	t.Log("configuring the testing environment")
	knativeAddon := knative.New()
	builder := environments.NewBuilder().WithAddons(knativeAddon)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 1)

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying the test environment becomes ready for use")
	waitForObjects, ready, err := env.Ready(ctx)
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that the knative addon has become ready for use")
	waitForObjects, ready, err = knativeAddon.Ready(ctx, env.Cluster())
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that knative deployments are up and running")
	deploymentList, err := env.Cluster().Client().AppsV1().Deployments(knative.DefaultNamespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	foundDeployments := map[string]bool{
		"activator":  false,
		"autoscaler": false,
		"controller": false,
		"webhook":    false,
	}
	for _, deployment := range deploymentList.Items {
		t.Logf("found knative deployment %s", deployment.Name)
		require.Equal(t, *deployment.Spec.Replicas, deployment.Status.ReadyReplicas)
		foundDeployments[deployment.Name] = true
	}
	for deploymentName, found := range foundDeployments {
		require.True(t, found, fmt.Sprintf("found deployment %s", deploymentName))
	}

	t.Log("cleaning up the knative addon")
	require.NoError(t, env.Cluster().DeleteAddon(ctx, knativeAddon))
	assert.Len(t, env.Cluster().ListAddons(), 0)

	t.Log("ensuring that the knative addon is cleaned up successfully")
	_, err = env.Cluster().Client().CoreV1().Namespaces().Get(ctx, knative.DefaultNamespace, metav1.GetOptions{})
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err))
}
