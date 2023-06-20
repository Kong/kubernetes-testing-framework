//go:build integration_tests

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kuma"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestEnvironmentWithKuma(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment")
	addon := kuma.New()
	builder := environments.NewBuilder().WithAddons(addon)

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

	t.Log("verifying that the kuma addon has become ready for use")
	waitForObjects, ready, err = addon.Ready(ctx, env.Cluster())
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that kuma deployments are up and running")
	require.Eventually(t, func() bool {
		foundDeployments := map[string]bool{
			"kuma-control-plane": false,
		}
		deploymentList, err := env.Cluster().Client().AppsV1().Deployments(kuma.Namespace).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, deployment := range deploymentList.Items {
				if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas {
					foundDeployments[deployment.Name] = true
				}
			}
		}
		for _, found := range foundDeployments {
			if !found {
				return false
			}
		}
		return true
	}, time.Minute, time.Second*5)

	t.Log("cleaning up the kuma addon")
	require.NoError(t, env.Cluster().DeleteAddon(ctx, addon))
	assert.Len(t, env.Cluster().ListAddons(), 0)

	t.Log("ensuring that the kuma addon is cleaned up successfully")
	deploys, err := env.Cluster().Client().AppsV1().Deployments(kuma.Namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, deploys.Items, 0)
}
