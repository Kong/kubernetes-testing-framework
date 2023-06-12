//go:build integration_tests
// +build integration_tests

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

func TestKindWithClusterVersion(t *testing.T) {
	t.Parallel()

	clusterVersion, err := semver.Parse("1.26.4")
	require.NoError(t, err)
	t.Logf("deploying a kind cluster with kubernetes version %s", clusterVersion)
	builder := kind.NewBuilder().WithClusterVersion(clusterVersion)
	cluster, err := builder.Build(context.Background())
	require.NoError(t, err)

	defer func() {
		t.Logf("cleaning up kind cluster %s", cluster.Name())
		// Don't use test context as it may be cancelled already
		assert.NoError(t, cluster.Cleanup(context.Background()))
	}()

	t.Log("waiting for cluster api to become usable")
	require.Eventually(t, func() bool {
		_, err := cluster.Client().ServerVersion()
		return err == nil
	}, time.Minute, time.Second)

	t.Logf("verifying that the created cluster is kubernetes version %s", clusterVersion)
	serverVersion, err := cluster.Version()
	require.NoError(t, err)
	require.Equal(t, clusterVersion.String(), serverVersion.String())
	require.True(t, clusterVersion.EQ(serverVersion))
}
