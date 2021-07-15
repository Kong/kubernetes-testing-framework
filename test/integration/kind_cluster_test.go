//+build integration_tests

package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

func TestEnvWithKindCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder()

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

	t.Log("verifying that the kong addon deployed both proxy and controller")
	kongDeployment, err := env.Cluster().Client().AppsV1().Deployments(kongAddonRaw.Namespace()).Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, kongDeployment.Spec.Template.Spec.Containers, 2)
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[0].Name, "ingress-controller")
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[1].Name, "proxy")

	t.Log("deploying a test deployment to ensure the environment's cluster is working")
	container := generators.NewContainer("httpbin", "docker.io/kennethreitz/httpbin", 80)
	deployment := generators.NewDeploymentForContainer(container)
	deployment, err = env.Cluster().Client().AppsV1().Deployments(corev1.NamespaceDefault).Create(ctx, deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("verifying the underlying pods deploy successfully")
	require.Eventually(t, func() bool {
		deployment, err = env.Cluster().Client().AppsV1().Deployments(corev1.NamespaceDefault).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	}, time.Minute*1, time.Second*1)

	t.Logf("exposing deployment %s via service", deployment.Name)
	service := generators.NewServiceForDeployment(deployment, corev1.ServiceTypeLoadBalancer)
	service, err = env.Cluster().Client().CoreV1().Services(corev1.NamespaceDefault).Create(ctx, service, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Logf("creating an ingress for service %s with ingress.class kong", service.Name)
	ingress := generators.NewIngressForService("/httpbin", map[string]string{
		"kubernetes.io/ingress.class": "kong",
		"konghq.com/strip-path":       "true",
	}, service)
	ingress, err = env.Cluster().Client().NetworkingV1().Ingresses(corev1.NamespaceDefault).Create(ctx, ingress, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Logf("waiting for ingress status update to validate that the kong controller is functioning")
	require.Eventually(t, func() bool {
		ingress, err = env.Cluster().Client().NetworkingV1().Ingresses(corev1.NamespaceDefault).Get(ctx, ingress.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return len(ingress.Status.LoadBalancer.Ingress) > 0
	}, time.Minute*1, time.Second*1)

	t.Logf("accessing the deployment via ingress %s to validate that the kong proxy is functioning", ingress.Name)
	httpc := http.Client{Timeout: time.Second * 3}
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("%s/httpbin", proxyURL))
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
	}, time.Minute*1, time.Second*1)
}

func TestEnvWithKindClusterKongProxyOnlyMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder()

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
