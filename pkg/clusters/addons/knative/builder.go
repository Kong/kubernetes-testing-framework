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

// WithVersion sets the addon version
func (b *Builder) WithVersion(version string) (*Builder, error) {
	if len(version) == 0 {
		return nil, errors.New("no version provided")
	}
	b.version = version
	return b, nil
}

// Build creates a knative addon using the builder parameters
func (b *Builder) Build() *addon {
	return &addon{
		version: b.version,
	}
}
