package environment

import (
	"fmt"

	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
)

// Builder is a toolkit for building a new test Environment.
type Builder struct {
	addons map[string]cluster.Addon
}

// WithAddons includes any provided Addon components in the cluster
// after the cluster is deployed.
func (b *Builder) WithAddons(addons ...cluster.Addon) *Builder {
	for _, addon := range addons {
		b.addons[addon.Name()] = addon
	}
	return b
}

func (b *Builder) Create() (Environment, error) {
	// FIXME: this is all using the old code, but need options passers e.t.c.
	// kind.ClusterConfigurationWithKongProxy{}
	// TODO: right now we simply default to a kind cluster
	return nil, fmt.Errorf("unimplemented")
}
