package loadimage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

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

	switch ctype := cluster.Type(); ctype {
	case kind.KindClusterType:
		if err := a.loadIntoKind(cluster); err != nil {
			return err
		}
	default:
		return fmt.Errorf("loadimage addon is not supported by cluster type '%v'", cluster.Type())
	}

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
