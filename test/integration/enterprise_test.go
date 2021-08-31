//+build integration_tests

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKongEnterpriseWithDBLessMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	t.Log("configuring the testing environment")
	metallb := metallbaddon.New()
	kong := kongaddon.NewBuilder().WithEnterprise(kongaddon.DBLESS).WithImage(kongaddon.EnterpriseImageRepo, kongaddon.EnterpriseImageTag).Build()
	builder := environment.NewBuilder().WithAddons(kong, metallb)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("waiting for environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Logf("verifying that the kong proxy service %s gets provisioned an IP address by metallb", kongaddon.DefaultProxyServiceName)
	proxyURL, err := kong.ProxyURL(ctx, env.Cluster())
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
}
