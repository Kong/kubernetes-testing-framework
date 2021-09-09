//+build integration_tests

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKongEnterprisePostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	t.Log("configuring the testing environment")
	metallb := metallbaddon.New()
	kong := kongaddon.NewBuilder().WithEnterprise().WithPostgreSQL().WithImage(kongaddon.DefaultEnterpriseImageRepo, kongaddon.DefaultEnterpriseImageTag).Build()
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

	adminURL, err := kong.ProxyAdminURL(ctx, env.Cluster())
	require.NoError(t, err)
	require.NotNil(t, adminURL)
	url := adminURL.String() + "/workspaces"
	fmt.Println("URL:>", url)

	var jsonStr = []byte(`{"name": "test-workspace"}`)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("kong-admin-token", "password")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	require.Equal(t, resp.StatusCode, 201)

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
