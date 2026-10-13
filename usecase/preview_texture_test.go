package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func TestPreviewTexture(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte{0xbb, 0xbb}, nil }}
	tex := &stubTex{meta: &domain.Texture{Width: 64, Height: 32, Format: domain.FmtDXT5}, png: []byte("PNG")}
	meta, png, err := NewPreviewTexture(repo, tex).Do(context.Background(), "g.res", "x.tex")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if meta.Width != 64 || meta.Format != domain.FmtDXT5 || string(png) != "PNG" {
		t.Errorf("got %v %q", meta, png)
	}
}

func TestPreviewTextureRejectsNonTex(t *testing.T) {
	repo := &fakeRepo{}
	_, _, err := NewPreviewTexture(repo, &stubTex{}).Do(context.Background(), "g.res", "x.set")
	if !errors.Is(err, domain.ErrNotTexture) {
		t.Errorf("err = %v, want ErrNotTexture", err)
	}
	if len(repo.replacedLog()) != 0 {
		t.Error("unexpected repo mutation")
	}
}

func TestPreviewTextureDecodeError(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte("junk"), nil }}
	tex := &stubTex{err: domain.ErrTextureHeader}
	_, _, err := NewPreviewTexture(repo, tex).Do(context.Background(), "g.res", "x.tex")
	if !errors.Is(err, domain.ErrTextureHeader) {
		t.Errorf("err = %v, want wrapped ErrTextureHeader", err)
	}
}
