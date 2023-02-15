//go:build integration_tests
// +build integration_tests

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	gokong "github.com/kong/go-kong/kong"
	"github.com/sethvargo/go-password/password"
	"github.com/stretchr/testify/require"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKongEnterprisePostgres(t *testing.T) {
	t.Parallel()

	t.Log("preparing kong enterprise secrets")
	licenseJSON, err := kong.GetLicenseJSONFromEnv()
	require.NoError(t, err)
	adminPassword, err := password.Generate(10, 5, 0, false, false)
	require.NoError(t, err)

	t.Log("configuring the testing environment")
	metallbAddon := metallbaddon.New()
	kongAddon := kongaddon.NewBuilder().
		WithProxyEnterpriseEnabled(licenseJSON).
		WithPostgreSQL().
		WithProxyAdminServiceTypeLoadBalancer().
		WithProxyEnterpriseSuperAdminPassword(adminPassword).
		Build()
	builder := environment.NewBuilder().WithAddons(kongAddon, metallbAddon)

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
	proxyURL, err := kongAddon.ProxyURL(ctx, env.Cluster())
	require.NoError(t, err)
	require.NotNil(t, proxyURL)

	t.Log("gathering the proxy admin URL")
	adminURL, err := kongAddon.ProxyAdminURL(ctx, env.Cluster())
	require.NoError(t, err)
	require.NotNil(t, adminURL)

	t.Log("building a GET request to gather admin api information")
	req, err := http.NewRequestWithContext(ctx, "GET", adminURL.String(), nil)
	require.NoError(t, err)
	req.Header.Set("Kong-Admin-Token", adminPassword)

	t.Run("verifying the admin api version is enterprise", func(t *testing.T) {
		t.Log("pulling the admin api information")
		httpc := http.Client{Timeout: time.Second * 10}
		var body []byte
		require.Eventually(t, func() bool {
			resp, err := httpc.Do(req)
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return false
			}
			t.Logf("RESPONSE CODE: %d STATUS: %s", resp.StatusCode, resp.Status)
			return resp.StatusCode == http.StatusOK
		}, time.Minute, time.Second)

		adminOutput := struct {
			Version string `json:"version"`
		}{}
		require.NoError(t, json.Unmarshal(body, &adminOutput))
		v, err := gokong.NewVersion(adminOutput.Version)
		require.NoError(t, err)

		t.Logf("admin api version %s", v)
		require.Truef(t, v.IsKongGatewayEnterprise(), "version %s should be an enterprise version but wasn't", v)
	})

	t.Log("verifying enterprise workspace API functionality")
	workspaceEnabledProxyURL := adminURL.String() + "/workspaces"
	jsonStr := []byte(`{"name": "test-workspace"}`)
	req, err = http.NewRequest("POST", workspaceEnabledProxyURL, bytes.NewBuffer(jsonStr))
	require.NoError(t, err)
	req.Header.Set("kong-admin-token", adminPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 10}
	require.Eventually(t, func() bool {
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusCreated
	}, time.Minute, time.Second)

	t.Log("deploying httpbin and waiting for readiness")
	httpbinAddon := httpbin.New()
	require.NoError(t, env.Cluster().DeployAddon(ctx, httpbinAddon))
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("accessing httpbin via ingress to validate that the kong proxy is functioning")
	httpc := http.Client{Timeout: time.Second * 10}
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("%s/%s", proxyURL, httpbinAddon.Path()))
		if err != nil {
			t.Logf("WARNING: error while waiting for %s: %v", proxyURL, err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			b := new(bytes.Buffer)
			n, err := b.ReadFrom(resp.Body)
			require.NoError(t, err)
			require.True(t, n > 0)
			return strings.Contains(b.String(), "<title>httpbin.org</title>")
		}
		return false
	}, time.Minute*3, time.Second)
}
