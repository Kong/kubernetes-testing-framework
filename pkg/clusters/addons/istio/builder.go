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
	grafanaEnabled    bool
	jaegerEnabled     bool
	kialiEnabled      bool
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

// WithPrometheus triggers a deployment of Prometheus configured specifically for Istio.
//
// See: https://istio.io/latest/docs/ops/integrations/prometheus/
func (b *Builder) WithPrometheus() *Builder {
	b.prometheusEnabled = true
	return b
}

// WithGrafana triggers a deployment of Grafana configured specifically for Istio.
//
// See: https://istio.io/latest/docs/ops/integrations/grafana/
func (b *Builder) WithGrafana() *Builder {
	b.grafanaEnabled = true
	return b
}

// WithJaeger triggers a deployment of Jaeger configured specifically for Istio.
//
// See: https://istio.io/latest/docs/ops/integrations/jaeger/
func (b *Builder) WithJaeger() *Builder {
	b.jaegerEnabled = true
	return b
}

// WithKiali triggers a deployment of Kiali configured specifically for Istio.
//
// See: https://kiali.io/documentation/
func (b *Builder) WithKiali() *Builder {
	b.kialiEnabled = true
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	return &Addon{
		name:         b.name,
		istioVersion: b.istioVersion,

		prometheusEnabled: b.prometheusEnabled,
		grafanaEnabled:    b.grafanaEnabled,
		jaegerEnabled:     b.jaegerEnabled,
		kialiEnabled:      b.kialiEnabled,
	}
}
