package usecase

import (
	"context"
	"fmt"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// PreviewTexture decodes one .tex entry to PNG + metadata.
type PreviewTexture struct {
	repo ArchiveRepo
	tex  TextureDecoder
}

// NewPreviewTexture wires the interactor.
func NewPreviewTexture(repo ArchiveRepo, tex TextureDecoder) *PreviewTexture {
	return &PreviewTexture{repo: repo, tex: tex}
}

// Do returns the texture descriptor and PNG bytes.
func (x *PreviewTexture) Do(ctx context.Context, archive, entry string) (*domain.Texture, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("preview_texture: %w", err)
	}
	if domain.KindOf(entry) != domain.KindTex {
		return nil, nil, fmt.Errorf("preview_texture %s/%s: %w", archive, entry, domain.ErrNotTexture)
	}
	data, err := x.repo.Read(archive, entry)
	if err != nil {
		return nil, nil, fmt.Errorf("preview_texture %s/%s: %w", archive, entry, err)
	}
	meta, png, err := x.tex.Decode(data)
	if err != nil {
		return nil, nil, fmt.Errorf("preview_texture %s/%s: texture: %w", archive, entry, err)
	}
	return meta, png, nil
}
