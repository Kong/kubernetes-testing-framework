package integration

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

var errBrokenAddonDeploy = errors.New("can't deploy purposely broken addon")

type brokenAddon struct{}

var _ clusters.Addon = &brokenAddon{}

func (a *brokenAddon) Name() clusters.AddonName {
	return "purposely broken addon"
}
func (a *brokenAddon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}
func (a *brokenAddon) Deploy(_ context.Context, _ clusters.Cluster) error {
	return errBrokenAddonDeploy
}
func (a *brokenAddon) Delete(_ context.Context, _ clusters.Cluster) error {
	return nil
}

func (a *brokenAddon) DumpDiagnostics(_ context.Context, _ clusters.Cluster) (map[string][]byte, error) {
	return nil, nil
}

func (a *brokenAddon) Ready(_ context.Context, _ clusters.Cluster) (waitingForObjects []runtime.Object, ready bool, err error) {
	return nil, false, nil
}

func TestKindClusterCleanupWhenErrorDuringCreationOccurs(t *testing.T) {
	builder := environments.NewBuilder()
	name := uuid.NewString()
	ctx := context.Background()

	t.Logf("creating kind cluster %q with broken addon - verify error expected during creation", name)
	_, err := builder.WithName(name).WithAddons(&brokenAddon{}).Build(ctx)
	require.ErrorIs(t, err, errBrokenAddonDeploy)

	t.Logf("ensuring kind cluster %q was cleaned up", name)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	require.NoErrorf(t, cmd.Run(), "stdout: %q stderr: %q", stdout.String(), stderr.String())
	require.NotContains(t, stdout.String(), name)
}
