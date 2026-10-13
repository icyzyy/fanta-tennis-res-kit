package usecase

import (
	"context"
	"fmt"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// PreviewImage decodes one image entry (.tex/.dds/.bmp/.tga) to PNG +
// metadata. Unlike PreviewTexture it carries no KindOf gate: extension
// dispatch lives in the ImageDecoder, which wraps ErrNotTexture itself.
type PreviewImage struct {
	repo ArchiveRepo
	dec  ImageDecoder
}

// NewPreviewImage wires the interactor.
func NewPreviewImage(repo ArchiveRepo, dec ImageDecoder) *PreviewImage {
	return &PreviewImage{repo: repo, dec: dec}
}

// Do returns the image descriptor and PNG bytes.
func (x *PreviewImage) Do(ctx context.Context, archive, entry string) (*domain.Texture, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("preview_image: %w", err)
	}
	data, err := x.repo.Read(archive, entry)
	if err != nil {
		return nil, nil, fmt.Errorf("preview_image %s/%s: %w", archive, entry, err)
	}
	meta, png, err := x.dec.Decode(entry, data)
	if err != nil {
		return nil, nil, fmt.Errorf("preview_image %s/%s: image: %w", archive, entry, err)
	}
	return meta, png, nil
}
