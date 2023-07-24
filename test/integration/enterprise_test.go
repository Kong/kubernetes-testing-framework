//go:build integration_tests

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
	SkipEnterpriseTestIfNoEnv(t)
	t.Parallel()

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
	eventuallyExpectedStatusCodeAndBody(t, httpClient, req, http.StatusOK, func(t *testing.T, body []byte) bool {
		t.Log("check expected enterprise version")
		adminOutput := struct {
			Version string `json:"version"`
		}{}
		if err := json.Unmarshal(body, &adminOutput); err != nil {
			t.Logf("WARNING: error while unmarshalling admin api output %s: %v", body, err)
			return false
		}
		v, err := gokong.NewVersion(adminOutput.Version)
		if err != nil {
			t.Logf("WARNING: error while parsing admin api version %s: %v", adminOutput.Version, err)
			return false
		}
		t.Logf("admin api version %s", v)
		if !v.IsKongGatewayEnterprise() {
			t.Logf("version %s should be an enterprise version but wasn't", v)
			return false
		}
		return true
	})

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
	eventuallyExpectedStatusCodeAndBody(t, httpClient, req, http.StatusOK, func(t *testing.T, body []byte) bool {
		const expectedBody = "<title>httpbin.org</title>"
		t.Logf("check expected content %s of the response body", expectedBody)
		return bytes.Contains(body, []byte(expectedBody))
	})

	const workspaceToCreate = "test-workspace"
	if adminPassword != "" {
		t.Log("verifying enterprise workspace API functionality using /workspaces (works only for dbmode)")
		req, err = http.NewRequestWithContext(
			ctx, http.MethodPost, adminURL.JoinPath("/workspaces").String(),
			strings.NewReader(fmt.Sprintf(`{"name": "%s"}`, workspaceToCreate)),
		)
		require.NoError(t, err)
		decorateRequestWithAdminPassword(t, req, adminPassword)
	} else {
		t.Log("verifying enterprise workspace API functionality using /config (works only for dblessmode)")
		t.Fatal("not implemented yet")
	}
	req.Header.Set("Content-Type", "application/json")
	eventuallyExpectedStatusCodeAndBody(t, httpClient, req, http.StatusCreated, nil)

	t.Log("verifying that the workspace was indeed created")
	req, err = http.NewRequestWithContext(
		ctx, http.MethodGet,
		adminURL.JoinPath("/workspaces/").JoinPath(workspaceToCreate).String(),
		nil,
	)
	require.NoError(t, err)
	if adminPassword != "" {
		decorateRequestWithAdminPassword(t, req, adminPassword)
	}
	eventuallyExpectedStatusCodeAndBody(t, httpClient, req, http.StatusOK, nil)

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

func eventuallyExpectedStatusCodeAndBody(
	t *testing.T, httpClient *http.Client, req *http.Request, expectedStatusCode int, bodyChecker func(t *testing.T, body []byte) bool,
) {
	require.Eventually(
		t,
		func() bool {
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Logf("WARNING: error while waiting for %s: %v", req.URL, err)
				return false
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Logf("WARNING: error cannot read response body %s: %v", req.URL, err)
				return false
			}
			if resp.StatusCode != expectedStatusCode {
				t.Logf("WARNING: unexpected response %s: %s with body: %s", req.URL, resp.Status, body)
				return false
			}
			if bodyChecker != nil && !bodyChecker(t, body) {
				t.Logf("WARNING: unexpected content of response body %s: %s", req.URL, body)
				return false
			}
			return true
		},
		time.Minute, time.Second,
	)
}
