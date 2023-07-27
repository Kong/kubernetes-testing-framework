//go:build integration_tests

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	argoaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/argocd"
	kongargoaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kongargo"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKongArgoAddon(t *testing.T) {
	namespace := "ktf-test-kong-addon"
	release := "integration"
	t.Log("configuring argo addon")
	argo := argoaddon.NewBuilder().Build()

	t.Log("configuring kong addon")
	kong := kongargoaddon.NewBuilder().
		WithNamespace(namespace).
		WithRelease(release).
		Build()

	t.Log("configuring the testing environment")
	// Add metallb to get an IP address for Kong's LoadBalancerservices
	// TODO: when https://github.com/Kong/kubernetes-testing-framework/issues/540
	// gets resolved then we can configure service types to be of ClusterIP and
	// do away with metallb deployment here.
	builder := environment.NewBuilder().WithAddons(argo, kong, metallbaddon.New())

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	err = <-env.WaitForReady(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 3)

	t.Log("verifying that the ArgoCD creates a Kong deployment and it reaches the ready state")
	require.Eventually(t, func() bool {
		_, ready, err := kong.Ready(ctx, env.Cluster())
		return err == nil && ready
	}, time.Minute*3, time.Second)
}
