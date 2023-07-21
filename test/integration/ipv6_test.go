//go:build integration_tests

package integration

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKindClusterWithIPv6(t *testing.T) {
	t.Parallel()

	t.Log("configuring the test environment with IPv6 only enabled")
	builder := environments.NewBuilder().WithIPv6Only().WithAddons(metallb.New())

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
}
