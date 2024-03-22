//go:build integration_tests

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestEnvironmentWithMetallb(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment")
	metallb := metallbaddon.New()
	kong := kongaddon.New()
	builder := environment.NewBuilder().WithAddons(kong, metallb)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 2)

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying the test environment becomes ready for use")
	waitForObjects, ready, err := env.Ready(ctx)
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that the metallb addon has become ready for use")
	waitForObjects, ready, err = metallb.Ready(ctx, env.Cluster())
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that the kong addon has become ready for use")
	waitForObjects, ready, err = kong.Ready(ctx, env.Cluster())
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Logf("verifying that the kong proxy service %s gets provisioned an IP address by metallb", kongaddon.DefaultProxyServiceName)
	proxyURL, err := kong.ProxyHTTPURL(ctx, env.Cluster())
	require.NoError(t, err)
	require.NotNil(t, proxyURL)

	t.Logf("found url %s for proxy, now verifying it is routable", proxyURL)
	httpc := http.Client{Timeout: time.Second * 3}
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(proxyURL.String())
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusNotFound
	}, time.Minute*1, time.Second*1)

	t.Log("cleaning up the metallb addon")
	require.NoError(t, env.Cluster().DeleteAddon(ctx, metallb))
	assert.Len(t, env.Cluster().ListAddons(), 1)

	t.Log("ensuring that the metallb addon is cleaned up successfully")
	require.Eventually(t, func() bool {
		_, err := env.Cluster().Client().CoreV1().Namespaces().Get(ctx, metallbaddon.DefaultNamespace, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true
			}
		}
		return false
	}, time.Minute*3, time.Second*1)
}
