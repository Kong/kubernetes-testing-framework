//go:build integration_tests

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/go-kong/kong"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/certmanager"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/registry"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

const httpbinImage = "docker.io/kennethreitz/httpbin"

func TestEnvironmentWithRegistryAddon(t *testing.T) {
	t.Skip("This test requires fixing: https://github.com/Kong/kubernetes-testing-framework/issues/1374")

	t.Parallel()

	t.Log("configuring the testing environment")
	registryAddon := registry.NewBuilder().WithServiceTypeLoadBalancer().Build()
	builder := environment.NewBuilder().WithAddons(
		metallb.New(),
		certmanager.New(),
		registryAddon,
	)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 3)

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying the test environment becomes ready for use")
	waitForObjects, ready, err := env.Ready(ctx)
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	t.Log("verifying that the addon was set up properly")
	require.NotEmpty(t, registryAddon.CertificatePEM())
	require.NotEmpty(t, registryAddon.LoadBalancerAddress())

	t.Log("creating a job to push an httpbin image to the registry")
	cfgmap, job := generators.GenerateBashJob("quay.io/podman/stable", "latest",
		fmt.Sprintf("podman image pull %s", httpbinImage),
		fmt.Sprintf("podman image push --tls-verify=false %s docker://%s/httpbin:latest", httpbinImage, registryAddon.LoadBalancerAddress()),
	)
	job.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{Privileged: kong.Bool(true)}

	t.Log("starting the podman image push job")
	_, err = env.Cluster().Client().CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(ctx, cfgmap, metav1.CreateOptions{})
	require.NoError(t, err)
	job, err = env.Cluster().Client().BatchV1().Jobs(corev1.NamespaceDefault).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("waiting for podman image push job to complete")
	require.Eventually(t, func() bool {
		job, err = env.Cluster().Client().BatchV1().Jobs(corev1.NamespaceDefault).Get(ctx, job.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return job.Status.Succeeded > 0
	}, time.Minute*5, time.Second)

	t.Log("creating a deployment using the image from the custom registry")
	container := corev1.Container{
		Name:  "httpbin",
		Image: fmt.Sprintf("%s/httpbin", registryAddon.LoadBalancerAddress()),
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		}},
	}
	deployment := generators.NewDeploymentForContainer(container)
	deployment, err = env.Cluster().Client().AppsV1().Deployments(corev1.NamespaceDefault).Create(ctx, deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("verify that the container image can be pulled and the pod starts")
	require.Eventually(t, func() bool {
		deployment, err = env.Cluster().Client().AppsV1().Deployments(corev1.NamespaceDefault).Get(ctx, deployment.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return deployment.Status.AvailableReplicas > 0
	}, time.Minute*3, time.Second)
}
