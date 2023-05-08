package knative

import (
	"errors"
)

// Builder constructs a knative addon
type Builder struct {
	version string
}

// NewBuilder returns a new Builder
func NewBuilder() *Builder {
	return &Builder{version: DefaultVersion}
}

// WithVersion sets the Knative version to deploy. The version must be a valid Knative git tag (e.g. `knative-v1.10.0`).
func (b *Builder) WithVersion(version string) (*Builder, error) {
	if len(version) == 0 {
		return nil, errors.New("no version provided")
	}
	b.version = version
	return b, nil
}

// Build creates a knative addon using the builder parameters
func (b *Builder) Build() *Addon {
	return &Addon{
		version: b.version,
	}
}
