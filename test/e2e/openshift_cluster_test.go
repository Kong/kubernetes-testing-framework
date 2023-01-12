//go:build e2e_tests
// +build e2e_tests

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/openshift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenShiftCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	t.Log("creating a new CRC VM")
	builder := openshift.NewBuilder()
	cluster, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up cleanup for cluster %s", cluster.Name())
	defer func() {
		t.Logf("running cluster cleanup for %s", cluster.Name())
		assert.NoError(t, cluster.Cleanup(ctx))
	}()
}
