//go:build integration_tests

package integration

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKindClusterWithIPv6(t *testing.T) {
	t.Parallel()

	t.Log("configuring the test environment with IPv6 only enabled")
	builder := environments.NewBuilder().WithIPv6Only().WithAddons(metallb.New(), kong.New())

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Log("waiting for the testing environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))
	defer func() { assert.NoError(t, env.Cleanup(ctx)) }()

	endpoints, err := env.Cluster().Client().CoreV1().Endpoints(corev1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			parsed := net.ParseIP(addr.IP)
			require.Nil(t, parsed.To4())
		}
	}

	require.Eventually(t, func() bool {
		kongServices := env.Cluster().Client().CoreV1().Services(kong.DefaultNamespace)
		service, err := kongServices.Get(ctx, kong.DefaultProxyServiceName, metav1.GetOptions{})
		if err != nil {
			return false
		}

		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return false
		}

		if net.ParseIP(service.Status.LoadBalancer.Ingress[0].IP).To4() != nil {
			return false
		}
		kongURL := fmt.Sprintf("http://[%s]", service.Status.LoadBalancer.Ingress[0].IP)
		resp, err := http.Get(kongURL)
		// we don't care that the proxy has nothing to serve so long as we can talk to it and get a valid HTTP response
		if err == nil && resp.StatusCode == http.StatusNotFound {
			return true
		}
		return false

	}, time.Minute*3, time.Second)
}
