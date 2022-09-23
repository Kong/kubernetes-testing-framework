package loadimage

import (
	"errors"
)

// -----------------------------------------------------------------------------
// CertManager Addon - Builder
// -----------------------------------------------------------------------------

type Builder struct {
	images []string
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithImage(image string) (*Builder, error) {
	if len(image) == 0 {
		return nil, errors.New("no image provided")
	}
	b.images = append(b.images, image)
	return b, nil
}

func (b *Builder) Build() *Addon {
	return &Addon{
		images: b.images,
		loaded: false,
	}
}
