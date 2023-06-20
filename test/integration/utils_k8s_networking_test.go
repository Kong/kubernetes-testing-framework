//go:build integration_tests

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/networking"
)

func TestWaitForConnectionOnServicePort(t *testing.T) {
	t.Parallel()

	t.Log("creating a test environment with MetalLB to test helper function WaitForConnectionOnServicePort")
	env, err := environments.NewBuilder().WithAddons(metallbaddon.New()).Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("waiting for the test environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying that addon - MetalLB has been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 1)

	t.Logf("verifying that helper function WaitForConnectionOnServicePort can make a successful TCP connection to existing K8s service")
	// It's safe to assume that kube-dns service will be available in any K8s cluster.
	// It can be used for this test, because it supports DNS over TCP too.
	conErr := networking.WaitForConnectionOnServicePort(ctx, env.Cluster().Client(), "kube-system", "kube-dns", 53, 5*time.Second)
	require.NoError(t, conErr)
}
