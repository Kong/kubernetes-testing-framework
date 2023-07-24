//go:build integration_tests

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sethvargo/go-password/password"
	"github.com/stretchr/testify/require"

	"github.com/kong/kubernetes-testing-framework/internal/test"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	metallbaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKongEnterprisePostgres(t *testing.T) {
	SkipEnterpriseTestIfNoEnv(t)

	licenseJSON := prepareKongEnterpriseLicense(t)

	t.Logf("generating a random password for the proxy admin service (applies only for dbmode)")
	adminPassword := password.MustGenerate(10, 5, 0, false, false)

	t.Log("configuring the testing environment")
	kongAddon := kongaddon.NewBuilder().
		WithProxyAdminServiceTypeLoadBalancer().
		WithPostgreSQL().
		WithProxyEnterpriseEnabled(licenseJSON).
		WithProxyEnterpriseSuperAdminPassword(adminPassword).
		Build()

	deployAndTestKongEnterprise(t, kongAddon, adminPassword)
}

func TestKongEnterpriseDBLess(t *testing.T) {
	SkipEnterpriseTestIfNoEnv(t)

	licenseJSON := prepareKongEnterpriseLicense(t)

	t.Log("configuring the testing environment")
	kongAddon := kongaddon.NewBuilder().
		WithProxyAdminServiceTypeLoadBalancer().
		WithProxyEnterpriseEnabled(licenseJSON).
		Build()

	deployAndTestKongEnterprise(t, kongAddon, "")
}

// deployAndTestKongEnterprise deploys a Kong Enterprise cluster and tests it for basic functionality.
// It works for both DB-less and DB-mode deployments (configuration of kongAddon). For DB-less set adminPassword to "".
// It verifies that workspace (enterprise feature) can be successfully created.
func deployAndTestKongEnterprise(t *testing.T, kongAddon *kongaddon.Addon, adminPassword string) {
	metallbAddon := metallbaddon.New()
	builder := environment.NewBuilder().WithAddons(kongAddon, metallbAddon)
	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	t.Cleanup(func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	})

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adminURL.String(), nil)
	require.NoError(t, err)
	if adminPassword != "" {
		decorateRequestWithAdminPassword(t, req, adminPassword)
	} else {
		t.Log("skipping setting the admin api password (Kong-Admin-Token header)")
	}
	t.Log("verifying the admin api version is enterprise")
	httpClient := &http.Client{Timeout: time.Second * 10}
	test.EventuallyExpectResponse(t, httpClient, req, test.WithStatusCode(http.StatusOK), test.WithEnterpriseHeader())

	t.Log("deploying httpbin and waiting for readiness")
	httpBinAddon := httpbin.New()
	require.NoError(t, env.Cluster().DeployAddon(ctx, httpBinAddon))
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("accessing httpbin via ingress to validate that the kong proxy is functioning")
	req, err = http.NewRequestWithContext(
		ctx, http.MethodGet,
		proxyURL.JoinPath(httpBinAddon.Path()).String(),
		nil,
	)
	require.NoError(t, err)
	test.EventuallyExpectResponse(
		t, httpClient, req, test.WithStatusCode(http.StatusOK), test.WithBodyContains("<title>httpbin.org</title>"),
	)

	const consumerGroupToCreate = "test-consumer-group"
	if adminPassword != "" {
		t.Log("verifying enterprise consumer groups API functionality using /consumer_groups (works only for dbmode)")
		req, err = http.NewRequestWithContext(
			ctx, http.MethodPost, adminURL.JoinPath("/consumer_groups").String(),
			strings.NewReader(fmt.Sprintf(`{"name": "%s"}`, consumerGroupToCreate)),
		)
		require.NoError(t, err)
		decorateRequestWithAdminPassword(t, req, adminPassword)
	} else {
		t.Log("verifying enterprise consumer groups API functionality using /config (works only for dblessmode)")
		req, err = http.NewRequestWithContext(
			ctx, http.MethodPost, adminURL.JoinPath("/config").String(),
			strings.NewReader(fmt.Sprintf(`{"_format_version": "3.0", "consumer_groups": [{"name": "%s"}]}`, consumerGroupToCreate)),
		)
	}
	req.Header.Set("Content-Type", "application/json")
	test.EventuallyExpectResponse(t, httpClient, req, test.WithStatusCode(http.StatusCreated))

	t.Log("verifying that the workspace was indeed created")
	req, err = http.NewRequestWithContext(
		ctx, http.MethodGet,
		adminURL.JoinPath("/consumer_groups/").JoinPath(consumerGroupToCreate).String(),
		nil,
	)
	require.NoError(t, err)
	if adminPassword != "" {
		decorateRequestWithAdminPassword(t, req, adminPassword)
	}
	test.EventuallyExpectResponse(t, httpClient, req, test.WithStatusCode(http.StatusOK))

}

func decorateRequestWithAdminPassword(t *testing.T, req *http.Request, adminPassword string) {
	t.Helper()
	const passwdHeader = "Kong-Admin-Token"
	t.Logf("setting the admin api password (Kong-Admin-Token header %s)", passwdHeader)
	req.Header.Set(passwdHeader, adminPassword)
}

func prepareKongEnterpriseLicense(t *testing.T) string {
	t.Helper()
	t.Log("preparing kong enterprise license")
	licenseJSON, err := kong.GetLicenseJSONFromEnv()
	require.NoError(t, err)
	return licenseJSON
}
