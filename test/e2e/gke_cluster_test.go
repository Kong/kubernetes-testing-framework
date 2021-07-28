//+build e2e_tests

package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/gke"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

const (
	gkeCredsVar    = "GOOGLE_APPLICATION_CREDENTIALS"
	gkeProjectVar  = "GOOGLE_PROJECT"
	gkeLocationVar = "GOOGLE_LOCATION"
)

var (
	gkeCreds    = os.Getenv(gkeCredsVar)
	gkeProject  = os.Getenv(gkeProjectVar)
	gkeLocation = os.Getenv(gkeLocationVar)
)

func TestGKECluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)
	defer cancel()

	t.Log("configuring GKE cloud environment for tests")
	require.NotEmpty(t, gkeCredsVar, "%s not set", gkeCredsVar)
	require.NotEmpty(t, gkeProject, "%s not set", gkeProjectVar)
	require.NotEmpty(t, gkeLocation, "%s not set", gkeLocationVar)

	t.Logf("configuring the GKE cluster PROJECT=(%s) LOCATION=(%s)", gkeProject, gkeLocation)
	builder := gke.NewBuilder([]byte(gkeCreds), gkeProject, gkeLocation)
	builder.WithClusterMinorVersion(1, 17)

	t.Logf("building cluster %s (this can take some time)", builder.Name)
	cluster, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up cleanup for cluster %s", cluster.Name())
	defer func() {
		assert.NoError(t, cluster.Cleanup(ctx))
	}()

	t.Log("verifying that the cluster can be communicated with")
	version, err := cluster.Client().ServerVersion()
	require.NoError(t, err)
	t.Logf("server version found: %s", version)

	t.Log("loading the gke cluster into a testing environment and deploying kong addon")
	env, err := environments.NewBuilder().WithAddons(kong.New()).WithExistingCluster(cluster).Build(ctx)
	require.NoError(t, err)

	t.Log("waiting for addons to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying that the kong addon deployed both proxy and controller")
	kongAddon, err := env.Cluster().GetAddon("kong")
	require.NoError(t, err)
	kongAddonRaw, ok := kongAddon.(*kong.Addon)
	require.True(t, ok)
	proxyURL, err := kongAddonRaw.ProxyURL(ctx, env.Cluster())
	require.NoError(t, err)
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
	ingress := generators.NewLegacyIngressForService("/httpbin", map[string]string{
		"kubernetes.io/ingress.class": "kong",
		"konghq.com/strip-path":       "true",
	}, service)
	ingress, err = env.Cluster().Client().NetworkingV1beta1().Ingresses(corev1.NamespaceDefault).Create(ctx, ingress, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Logf("waiting for ingress status update to validate that the kong controller is functioning")
	require.Eventually(t, func() bool {
		ingress, err = env.Cluster().Client().NetworkingV1beta1().Ingresses(corev1.NamespaceDefault).Get(ctx, ingress.Name, metav1.GetOptions{})
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
