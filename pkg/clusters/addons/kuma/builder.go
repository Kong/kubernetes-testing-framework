package kuma

import (
	"io"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool to generate Kuma cluster addons.
type Builder struct {
	name    string
	version semver.Version
	logger  *logrus.Logger

	mtlsEnabled bool
}

// NewBuilder provides a new Builder object for configuring Kuma cluster addons.
func NewBuilder() *Builder {
	return &Builder{
		name: string(AddonName),
	}
}

// WithVersion configures the specific version of Kuma which should be deployed.
func (b *Builder) WithVersion(version semver.Version) *Builder {
	b.version = version
	return b
}

// WithLogger adds a logger that will provide extra information about the build step
// of the addon at various configured log levels.
func (b *Builder) WithLogger(logger *logrus.Logger) *Builder {
	b.logger = logger
	return b
}

// WithMTLS enables the MTLS policy on the default mesh
//
// See: https://kuma.io/docs/dev/policies/mutual-tls/
func (b *Builder) WithMTLS() *Builder {
	b.mtlsEnabled = true
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	if b.logger == nil {
		b.logger = &logrus.Logger{Out: io.Discard}
	}
	return &Addon{
		name:    b.name,
		version: b.version,
		logger:  b.logger,

		mtlsEnabled: b.mtlsEnabled,
	}
}
