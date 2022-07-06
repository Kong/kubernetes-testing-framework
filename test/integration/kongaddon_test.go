//go:build integration_tests
// +build integration_tests

package integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kongaddon "github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

type customImageTest struct {
	controllerImageRepo string
	controllerImageTag  string
	proxyImageRepo      string
	proxyImageTag       string
}

func (tc customImageTest) controllerImage() string {
	return fmt.Sprintf("%s:%s", tc.controllerImageRepo, tc.controllerImageTag)
}

func (tc customImageTest) proxyImage() string {
	return fmt.Sprintf("%s:%s", tc.proxyImageRepo, tc.proxyImageTag)
}

func (tc customImageTest) name() string {
	return fmt.Sprintf("KongAddonImages:[%s,%s]", tc.controllerImage(), tc.proxyImage())
}

func TestKongAddonWithCustomImage(t *testing.T) {
	tests := []customImageTest{
		{
			controllerImageRepo: "kong/kubernetes-ingress-controller",
			controllerImageTag:  "2.3.0",
			proxyImageRepo:      "kong",
			proxyImageTag:       "2.7",
		},
		{
			controllerImageRepo: "kong/kubernetes-ingress-controller",
			controllerImageTag:  "2.3.1",
			proxyImageRepo:      "kong",
			proxyImageTag:       "2.8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name(), func(t *testing.T) {
			t.Parallel()
			testKongAddonWithCustomImage(t, tc)
		})
	}
}

func testKongAddonWithCustomImage(t *testing.T, tc customImageTest) {
	t.Log("configuring kong addon with custom images")
	kong := kongaddon.NewBuilder().
		WithProxyImage(tc.proxyImageRepo, tc.proxyImageTag).
		WithControllerImage(tc.controllerImageRepo, tc.controllerImageTag).
		Build()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder().WithAddons(kong)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 1)

	t.Log("verifying that the kong deployment is using custom images")
	deployments := env.Cluster().Client().AppsV1().Deployments(kong.Namespace())
	kongDeployment, err := deployments.Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, kongDeployment.Spec.Template.Spec.Containers, 2)
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[0].Name, "ingress-controller")
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[1].Name, "proxy")
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[0].Image, tc.controllerImage())
	require.Equal(t, kongDeployment.Spec.Template.Spec.Containers[1].Image, tc.proxyImage())
}

func TestKongAddonWithPullSecret(t *testing.T) {
	t.Log("configuring kong addon with pull secret")
	kongPullUsername := os.Getenv("KTF_TEST_KONG_PULL_USERNAME")
	kongPullPassword := os.Getenv("KTF_TEST_KONG_PULL_PASSWORD")
	if kongPullUsername == "" || kongPullPassword == "" {
		t.Skip("either KTF_TEST_KONG_PULL_USERNAME or KTF_TEST_KONG_PULL_PASSWORD unset, skipping pull Secret test")
	}
	kong := kongaddon.NewBuilder().WithProxyImagePullSecret("", kongPullUsername, kongPullPassword, "").Build()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder().WithAddons(kong)

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("verifying that addons have been loaded into the environment")
	require.Len(t, env.Cluster().ListAddons(), 1)

	t.Log("verifying that the pull secret exists")
	secret, err := env.Cluster().Client().CoreV1().Secrets(kong.Namespace()).Get(ctx, kongaddon.ProxyPullSecretName, metav1.GetOptions{})
	require.NoError(t, err)

	deploy, err := env.Cluster().Client().AppsV1().Deployments(kong.Namespace()).Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, deploy.Spec.Template.Spec.ImagePullSecrets, 1)
	require.Equal(t, deploy.Spec.Template.Spec.ImagePullSecrets[0].Name, secret.Name)
}
