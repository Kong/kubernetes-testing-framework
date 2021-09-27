package istio

import (
	"github.com/blang/semver/v4"
)

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool to generate Istio cluster addons.
type Builder struct {
	name              string
	istioVersion      semver.Version
	prometheusEnabled bool
}

// NewBuilder provides a new Builder object for configuring Istio cluster addons.
func NewBuilder() *Builder {
	return &Builder{
		name: string(AddonName),
	}
}

// WithVersion configures the specific version of Istio which should be deployed.
func (b *Builder) WithVersion(version semver.Version) *Builder {
	b.istioVersion = version
	return b
}

// WithPrometheus triggers a deployment of Prometheus configured specifically with
// Istio in mind.
//
// See: https://istio.io/latest/docs/ops/integrations/prometheus/
func (b *Builder) WithPrometheus() *Builder {
	b.prometheusEnabled = true
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	return &Addon{
		name:              b.name,
		istioVersion:      b.istioVersion,
		prometheusEnabled: b.prometheusEnabled,
	}
}
