package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// TestPreviewImage checks the happy path: entry bytes reach the decoder
// unchanged (name + data), and its result passes through.
func TestPreviewImage(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte("tga-bytes"), nil }}
	dec := &stubImg{meta: &domain.Texture{Width: 60, Height: 60, Format: domain.FmtRGBA32}, png: []byte("PNG")}
	meta, png, err := NewPreviewImage(repo, dec).Do(context.Background(), "g.res", "Cursor_Normal.tga")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if dec.gotName != "Cursor_Normal.tga" || string(dec.gotData) != "tga-bytes" {
		t.Errorf("decoder got (%q, %q), want (Cursor_Normal.tga, tga-bytes)", dec.gotName, dec.gotData)
	}
	if meta.Width != 60 || meta.Format != domain.FmtRGBA32 || string(png) != "PNG" {
		t.Errorf("got %+v %q", meta, png)
	}
}

// TestPreviewImageRepoError catches a broken wrap: the repo error must
// survive errors.Is and carry the preview_image archive/entry context.
func TestPreviewImageRepoError(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return nil, domain.ErrNoSuchEntry }}
	_, _, err := NewPreviewImage(repo, &stubImg{}).Do(context.Background(), "g.res", "missing.tga")
	if !errors.Is(err, domain.ErrNoSuchEntry) {
		t.Errorf("err = %v, want wrapped ErrNoSuchEntry", err)
	}
	if !strings.Contains(err.Error(), "preview_image g.res/missing.tga") {
		t.Errorf("err = %v, want preview_image g.res/missing.tga context", err)
	}
}

// TestPreviewImageContextCanceled catches a missing ctx check: a cancelled
// context must fail before the repo is touched.
func TestPreviewImageContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) {
		t.Error("repo read after context cancellation")
		return nil, nil
	}}
	_, _, err := NewPreviewImage(repo, &stubImg{}).Do(ctx, "g.res", "x.tga")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want wrapped context.Canceled", err)
	}
}

// TestPreviewImageDecodeError catches a broken decode wrap: the decoder
// error must survive errors.Is and carry the image context.
func TestPreviewImageDecodeError(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte("junk"), nil }}
	_, _, err := NewPreviewImage(repo, &stubImg{err: domain.ErrNotTexture}).Do(context.Background(), "g.res", "x.xyz")
	if !errors.Is(err, domain.ErrNotTexture) {
		t.Errorf("err = %v, want wrapped ErrNotTexture", err)
	}
	if !strings.Contains(err.Error(), "preview_image g.res/x.xyz: image:") {
		t.Errorf("err = %v, want preview_image g.res/x.xyz: image: context", err)
	}
}
