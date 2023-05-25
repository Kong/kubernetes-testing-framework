//go:build integration_tests
// +build integration_tests

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/networking"
)

func TestKindClusterBasics(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder()

	// TODO https://github.com/Kong/kubernetes-testing-framework/issues/364 remove once metallb behaves again
	builder = builder.WithKubernetesVersion(semver.Version{Major: 1, Minor: 24, Patch: 4})

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.WithAddons(metallb.New(), kong.New()).Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that both addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 2)

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying the test environment becomes ready for use")
	waitForObjects, ready, err := env.Ready(ctx)
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Logf("pulling the kong addon from the environment's cluster to verify proxy URL")
	kongAddon, err := env.Cluster().GetAddon("kong")
	require.NoError(t, err)
	kongAddonRaw, ok := kongAddon.(*kong.Addon)
	require.True(t, ok)
	proxyURL, err := kongAddonRaw.ProxyURL(ctx, env.Cluster())
	require.NoError(t, err)

	t.Log("verifying the kong proxy is returning its default 404 response")
	httpc := http.Client{Timeout: time.Second * 10}
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(proxyURL.String())
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusNotFound
	}, time.Minute*3, time.Second)

	t.Log("verifying that the kong addon deployed both proxy and controller")
	kongDeployment, err := env.Cluster().Client().AppsV1().Deployments(kongAddonRaw.Namespace()).Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, kongDeployment.Spec.Template.Spec.Containers, 2)
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[0].Name, "ingress-controller")
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[1].Name, "proxy")

	t.Log("deploying httpbin addon to test http traffic")
	httpbinAddon := httpbin.New()
	require.NoError(t, env.Cluster().DeployAddon(ctx, httpbinAddon))

	t.Log("waiting for addon to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("accessing httpbin via ingress to validate that the kong proxy is functioning")
	httpbinURL := fmt.Sprintf("%s/%s/status/418", proxyURL.String(), httpbinAddon.Path())
	require.NoError(t, <-networking.WaitForHTTP(ctx, httpbinURL, 418))
}

func TestKindClusterCustomConfigReader(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment with custom configuration - max-endpoints-per-slice=2")
	// Be cautious to not include tabs in the YAML.
	cfg := strings.NewReader(`
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
- |
  apiVersion: kubeadm.k8s.io/v1beta3
  kind: ClusterConfiguration
  controllerManager:
    extraArgs:
      max-endpoints-per-slice: "2"`)

	env, err := environment.
		NewBuilder().
		WithClusterBuilder(kind.NewBuilder().WithConfigReader(cfg)).
		Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("deploying deployment with 3 replicas to test EndpointSlices custom configuration")
	const svcName = "httpbin"
	container := generators.NewContainer(svcName, httpbin.Image, httpbin.DefaultPort)
	deployment := generators.NewDeploymentForContainer(container)
	deployment.Spec.Replicas = lo.ToPtr[int32](3)
	deployment, err = env.Cluster().Client().AppsV1().Deployments(corev1.NamespaceDefault).Create(ctx, deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	service := generators.NewServiceForDeployment(deployment, corev1.ServiceTypeClusterIP)
	_, err = env.Cluster().Client().CoreV1().Services(corev1.NamespaceDefault).Create(ctx, service, metav1.CreateOptions{})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		eps, err := env.Cluster().Client().DiscoveryV1().EndpointSlices(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, svcName),
		})
		assert.NoErrorf(c, err, "failed to list EndpointSlices for service %s", svcName)
		// For each Pod endpoint entry will be created in EndpointSlice, thus for 3 replicas we expect 2 EndpointSlices.
		assert.Lenf(c, eps.Items, 2, "number of expected EndpointSlices does not match")
	},
		3*time.Minute, time.Second,
	)
}

func TestKindClusterProxyOnly(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder()

	// TODO https://github.com/Kong/kubernetes-testing-framework/issues/364 remove once metallb behaves again
	builder = builder.WithKubernetesVersion(semver.Version{Major: 1, Minor: 24, Patch: 4})

	t.Log("building the testing environment and Kubernetes cluster with the KIC controller disabled")
	env, err := builder.WithAddons(metallb.New(), kong.NewBuilder().WithControllerDisabled().Build()).Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that both addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 2)

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Logf("pulling the kong addon from the environment's cluster")
	kongAddon, err := env.Cluster().GetAddon("kong")
	require.NoError(t, err)
	kongAddonRaw, ok := kongAddon.(*kong.Addon)
	require.True(t, ok)

	t.Log("verifying that the kong addon disabled the controller")
	kongDeployment, err := env.Cluster().Client().AppsV1().Deployments(kongAddonRaw.Namespace()).Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, kongDeployment.Spec.Template.Spec.Containers, 1)
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[0].Name, "proxy")
}
