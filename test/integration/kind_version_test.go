//+build integration_tests

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

func TestKindClusterOlderVersion(t *testing.T) {
	ctx := context.Background()
	clusterVersion := semver.MustParse("1.20.7")

	t.Logf("deploying a kind cluster with kubernetes version %s", clusterVersion)
	builder := kind.NewBuilder().WithClusterVersion(clusterVersion)
	cluster, err := builder.Build(ctx)
	require.NoError(t, err)

	defer func() {
		t.Logf("cleaning up kind cluster %s", cluster.Name())
		assert.NoError(t, cluster.Cleanup(ctx))
	}()

	t.Log("waiting for cluster api to become usable")
	require.Eventually(t, func() bool {
		_, err := cluster.Client().ServerVersion()
		if err != nil {
			return false
		}
		return true
	}, time.Minute*1, time.Second*1)

	t.Logf("verifying that the created cluster is kubernetes version %s", clusterVersion)
	serverVersion, err := cluster.Client().ServerVersion()
	require.NoError(t, err)
	assert.Equal(t, "v"+clusterVersion.String(), serverVersion.String())
}

func TestKindClusterNewerVersion(t *testing.T) {
	ctx := context.Background()
	clusterVersion := semver.MustParse("1.21.1")

	t.Logf("deploying a kind cluster with kubernetes version %s", clusterVersion)
	builder := kind.NewBuilder().WithClusterVersion(clusterVersion)
	cluster, err := builder.Build(ctx)
	require.NoError(t, err)

	defer func() {
		t.Logf("cleaning up kind cluster %s", cluster.Name())
		assert.NoError(t, cluster.Cleanup(ctx))
	}()

	t.Log("waiting for cluster api to become usable")
	require.Eventually(t, func() bool {
		_, err := cluster.Client().ServerVersion()
		if err != nil {
			return false
		}
		return true
	}, time.Minute*1, time.Second*1)

	t.Logf("verifying that the created cluster is kubernetes version %s", clusterVersion)
	serverVersion, err := cluster.Client().ServerVersion()
	require.NoError(t, err)
	assert.Equal(t, "v"+clusterVersion.String(), serverVersion.String())
}
