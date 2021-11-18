package loadimage

import (
	"errors"
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

func (b *Builder) WithImage(image string) (*Builder, error) {
	if len(image) == 0 {
		return nil, errors.New("no image provided")
	}
	b.image = image
	return b, nil
}

func (b *Builder) Build() *Addon {
	return &Addon{
		image:  b.image,
		loaded: false,
	}
}
