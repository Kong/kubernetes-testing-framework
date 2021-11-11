package loadimage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// CertManager Addon - Builder
// -----------------------------------------------------------------------------

type Builder struct {
	image string
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithImage(image string) *Builder {
	b.image = image
	return b
}

func (b *Builder) Build() *Addon {
	return &Addon{
		image:  b.image,
		loaded: false,
	}
}

// -----------------------------------------------------------------------------
// CertManager Addon
// -----------------------------------------------------------------------------

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "loadimage"
)

type Addon struct {
	image  string
	loaded bool
}

func New() clusters.Addon {
	return &Addon{}
}

// -----------------------------------------------------------------------------
// CertManager Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	if a.image == "" {
		a.loaded = true
		return nil
	}
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("addon %v is not available on cluster types other than %v", a.Name(), kind.KindClusterType)
	}
	deployArgs := []string{
		"load", "docker-image",
		a.image,
		"--name", cluster.Name(),
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kind", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}
	a.loaded = true

	return nil
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// per https://github.com/kubernetes-sigs/kind/issues/658 this is basically impossible
	return fmt.Errorf("cannot remove loaded images from a cluster")
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	// no way to verify this, we rely on Deploy's cmd.Run() not failing
	return nil, a.loaded, nil
}
