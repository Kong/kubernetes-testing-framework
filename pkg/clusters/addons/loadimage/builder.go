package loadimage

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
