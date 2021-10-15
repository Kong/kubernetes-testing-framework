//go:build integration_tests
// +build integration_tests


package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/istio"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

func TestIstioAddonDeployment(t *testing.T) {
	t.Parallel()

	t.Log("deploying the test cluster and environment")
	metallbAddon := metallb.New()
	istioAddon := istio.NewBuilder().
		WithPrometheus().
		WithGrafana().
		WithJaeger().
		WithKiali().
		Build()
	env, err := environments.NewBuilder().WithAddons(metallbAddon, istioAddon).Build(ctx)
	require.NoError(t, err)

	t.Log("waiting for the environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Logf("istio version %s was deployed, verifying that istiod is up and running", istioAddon.Version().String())
	_, err = env.Cluster().Client().CoreV1().Namespaces().Get(ctx, istioAddon.Namespace(), metav1.GetOptions{})
	require.NoError(t, err)
	deployment, err := env.Cluster().Client().AppsV1().Deployments(istioAddon.Namespace()).Get(ctx, "istiod", metav1.GetOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, int32(1), deployment.Status.ReadyReplicas)

	t.Log("creating two namespaces to test Istio, one that's mesh enabled and one that is not")
	require.NoError(t, clusters.CreateNamespace(ctx, env.Cluster(), "without-istio"))
	require.NoError(t, clusters.CreateNamespace(ctx, env.Cluster(), "with-istio"))
	require.NoError(t, istioAddon.EnableMeshForNamespace(ctx, env.Cluster(), "with-istio"))

	t.Log("adding a test deployment into each namespace")
	container := generators.NewContainer("httpbin", httpbin.Image, 80)
	deployment = generators.NewDeploymentForContainer(container)
	for _, namespace := range []string{"without-istio", "with-istio"} {
		deployment.Namespace = namespace
		_, err = env.Cluster().Client().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	t.Log("verifying istio sidecar injection")
	require.Eventually(t, func() bool {
		// pull the pods from the istio enabled namespace to check the containers
		pods, err := env.Cluster().Client().CoreV1().Pods("with-istio").List(ctx, metav1.ListOptions{LabelSelector: "app=httpbin"})
		require.NoError(t, err)
		if len(pods.Items) < 1 {
			return false
		}
		require.Len(t, pods.Items, 1, "there should only be one pod in this namespace")

		// if the sidecar container hasn't been loaded yet, wait
		httpbinPod := pods.Items[0]
		if len(httpbinPod.Spec.Containers) < 2 {
			return false
		}

		// identify the side-loaded istio proxy container
		var istioProxyPodFound bool
		for _, container := range httpbinPod.Spec.Containers {
			if container.Name == "istio-proxy" {
				istioProxyPodFound = true
			}
		}

		return istioProxyPodFound
	}, time.Minute, time.Second)

	t.Log("performing some additional sanity checks")
	pods, err := env.Cluster().Client().CoreV1().Pods("without-istio").List(ctx, metav1.ListOptions{LabelSelector: "app=httpbin"})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1, "there should only be one pod in this namespace")
	httpbinPod := pods.Items[0]
	require.Len(t, httpbinPod.Spec.Containers, 1, "there should be no sidecard loaded for this pod")

	t.Log("deleting istio addon")
	require.NoError(t, istioAddon.Delete(ctx, env.Cluster()))
	_, err = env.Cluster().Client().CoreV1().Namespaces().Get(ctx, istioAddon.Namespace(), metav1.GetOptions{})
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err))
}
