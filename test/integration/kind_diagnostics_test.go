//go:build integration_tests

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	environment "github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKindDiagnosticDump(t *testing.T) {
	t.Parallel()

	t.Log("configuring the testing environment")
	builder := environment.NewBuilder()

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	t.Cleanup(func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		require.NoError(t, env.Cleanup(ctx))
	})

	t.Log("waiting for the test environment to be ready for use")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("verifying the test environment becomes ready for use")
	waitForObjects, ready, err := env.Ready(ctx)
	require.NoError(t, err)
	require.Len(t, waitForObjects, 0)
	require.True(t, ready)

	cluster := env.Cluster()

	t.Log("verifying that DumpDiagnostics functions as expected")
	output, err := cluster.DumpDiagnostics(ctx, t.Name())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(output))
	}()

	logsPath, _ := filepath.Glob(filepath.Join(output, fmt.Sprintf("%s-control-plane", cluster.Name()), "containers", "kindnet-*"))
	require.NotZero(t, len(logsPath))
	logs, err := os.ReadFile(logsPath[0])
	require.NoError(t, err)
	require.NotZero(t, len(logs))

	describe, err := os.ReadFile(filepath.Join(output, "kubectl_describe_all.txt"))
	require.NoError(t, err)
	require.NotZero(t, len(describe))

	get, err := os.ReadFile(filepath.Join(output, "kubectl_get_all.yaml"))
	require.NoError(t, err)
	require.NotZero(t, len(get))

	meta, err := os.ReadFile(filepath.Join(output, "meta.txt"))
	require.NoError(t, err)
	require.NotZero(t, len(meta))
	require.Contains(t, string(meta), t.Name())
}
