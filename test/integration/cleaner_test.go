//go:build integration_tests

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

const (
	// TODO: This should be using a similar mechanism as KIC does
	// https://github.com/Kong/kubernetes-ingress-controller/tree/b1db86381c2b9a0c9e7290181db397bfd2b319a2/test/internal/cmd/generate-gateway-api-consts
	gatewayAPIKustomizeURL = "github.com/kubernetes-sigs/gateway-api/config/crd/experimental?ref=v1.0.0"
)

func TestCleaner(t *testing.T) {
	ctx := context.Background()

	builder := environments.NewBuilder()
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	cluster := env.Cluster()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, gatewayv1.Install(scheme))
	cleaner := clusters.NewCleaner(cluster, scheme)
	t.Cleanup(func() { cleaner.Cleanup(context.Background()) })

	t.Log("waiting for the test environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	require.NoError(t, clusters.KustomizeDeployForCluster(ctx, cluster, gatewayAPIKustomizeURL))

	ns, err := clusters.GenerateNamespace(ctx, cluster, t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { cleaner.AddNamespace(ns) })
	// Don't add to cleaner now because we want to assert objects existence before
	// namespace is removed.
	t.Logf("created namespace for test: %s", ns.Name)

	t.Log("deploying a new configmap")
	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
			Annotations: map[string]string{
				clusters.TestResourceLabel: t.Name(),
			},
		},
		Data: map[string]string{
			"dummy": "data",
		},
	}
	cfg, err = cluster.Client().CoreV1().ConfigMaps(ns.Name).Create(ctx, cfg, metav1.CreateOptions{})
	require.NoError(t, err)
	cleaner.Add(cfg)

	t.Log("getting a gateway client")
	gatewayClient, err := gatewayclient.NewForConfig(cluster.Config())
	require.NoError(t, err)

	t.Log("deploying a new gatewayClass")
	gwc := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
			Annotations: map[string]string{
				clusters.TestResourceLabel: t.Name(),
			},
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: "konghq.com/kic-gateway-controller",
		},
	}
	gwc, err = gatewayClient.GatewayV1().GatewayClasses().Create(ctx, gwc, metav1.CreateOptions{})
	require.NoError(t, err)
	cleaner.Add(gwc)

	t.Log("deploying a new gateway")
	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
			Annotations: map[string]string{
				clusters.TestResourceLabel: t.Name(),
			},
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(gwc.Spec.ControllerName),
			Listeners: []gatewayv1.Listener{{
				Name:     "http",
				Protocol: gatewayv1.HTTPProtocolType,
				Port:     gatewayv1.PortNumber(80),
			}},
		},
	}

	gw, err = gatewayClient.GatewayV1().Gateways(ns.Name).Create(ctx, gw, metav1.CreateOptions{})
	require.NoError(t, err)
	cleaner.Add(gw)

	httproute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
			Annotations: map[string]string{
				clusters.TestResourceLabel: t.Name(),
			},
		},
	}

	httproute, err = gatewayClient.GatewayV1().HTTPRoutes(ns.Name).Create(ctx, httproute, metav1.CreateOptions{})
	require.NoError(t, err)
	cleaner.Add(httproute)

	t.Log("verify objects actually exist")
	cfg, err = cluster.Client().CoreV1().ConfigMaps(ns.Name).Get(ctx, cfg.Name, metav1.GetOptions{})
	require.NoError(t, err)
	gwc, err = gatewayClient.GatewayV1().GatewayClasses().Get(ctx, gwc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	gw, err = gatewayClient.GatewayV1().Gateways(ns.Name).Get(ctx, gw.Name, metav1.GetOptions{})
	require.NoError(t, err)
	httproute, err = gatewayClient.GatewayV1().HTTPRoutes(ns.Name).Get(ctx, httproute.Name, metav1.GetOptions{})
	require.NoError(t, err)

	require.NoError(t, cleaner.Cleanup(context.Background()))

	t.Log("verify objects actually got removed")
	// TODO: Cleaner should also clean the config map but its Kind and APIVersion are also empty.
	// Possibly related:
	// - https://github.com/kubernetes/kubernetes/issues/3030
	// - https://github.com/kubernetes/kubernetes/issues/80609
	// cfg, err = cluster.Client().CoreV1().ConfigMaps(ns.Name).Get(ctx, cfg.Name, metav1.GetOptions{})
	// require.Error(t, err)
	// require.Truef(t, errors.IsNotFound(err), "configmap should be deleted at this point by the cleaner: %v", err)

	gwc, err = gatewayClient.GatewayV1().GatewayClasses().Get(ctx, gwc.Name, metav1.GetOptions{})
	require.Error(t, err)
	require.Truef(t, errors.IsNotFound(err), "gatewayclass should be deleted at this point by the cleaner: %v", err)

	gw, err = gatewayClient.GatewayV1().Gateways(ns.Name).Get(ctx, gw.Name, metav1.GetOptions{})
	require.Error(t, err)
	require.Truef(t, errors.IsNotFound(err), "gateway should be deleted at this point by the cleaner: %v", err)

	httproute, err = gatewayClient.GatewayV1().HTTPRoutes(ns.Name).Get(ctx, httproute.Name, metav1.GetOptions{})
	require.Error(t, err)
	require.Truef(t, errors.IsNotFound(err), "httproute should be deleted at this point by the cleaner: %v", err)
}
