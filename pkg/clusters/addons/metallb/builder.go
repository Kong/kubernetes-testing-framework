package metallb

// -----------------------------------------------------------------------------
// Metallb Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool for metallb cluster.Addons.
type Builder struct {
	disablePoolCreation bool
}

// NewBuilder provides a new Builder object with default addon settings.
func NewBuilder() *Builder {
	builder := &Builder{
		disablePoolCreation: false,
	}
	return builder
}

// WithIPAddressPoolDisabled instructs the builder to create addons with pool creation disabled.
func (b *Builder) WithIPAddressPoolDisabled() *Builder {
	b.disablePoolCreation = true
	return b
}

// Build generates an addon with the builder's configuration.
func (b *Builder) Build() *Addon {
	return &Addon{
		disablePoolCreation: b.disablePoolCreation,
	}
}
