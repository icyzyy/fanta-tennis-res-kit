package texdecoder

import (
	"bytes"
	"errors"
	"image/png"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
	"github.com/icyzyy/fanta-tennis-res-kit/usecase"
)

var _ usecase.TextureDecoder = (*Decoder)(nil)

func TestDecodeRawFixturesAgainstOracle(t *testing.T) {
	for _, fx := range texFixtures {
		if fx.format != domain.FmtRGB24 && fx.format != domain.FmtRGBA32 {
			continue
		}
		t.Run(fx.file, func(t *testing.T) {
			meta, pix, err := parse(fixtureTex(t, fx.file))
			if err != nil {
				t.Fatal(err)
			}
			cmpPixels(t, fx.file, decodeRaw(pix, meta.Width, meta.Height, fx.format))
		})
	}
}

// TestDecodeAllFixtures drives the full port: every staged .tex must come
// out as a decodable PNG pixel-identical to its committed oracle.
func TestDecodeAllFixtures(t *testing.T) {
	for _, fx := range texFixtures {
		t.Run(fx.file, func(t *testing.T) {
			meta, pngBytes, err := New().Decode(fixtureTex(t, fx.file))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(pngBytes))
			if err != nil {
				t.Fatalf("output is not a PNG: %v", err)
			}
			got := toNRGBA(img)
			want := oracleRGBA(t, fx.file)
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Error("decoded pixels differ from oracle")
			}
			if meta.Width != want.Bounds().Dx() || meta.Height != want.Bounds().Dy() {
				t.Errorf("meta %dx%d != oracle %dx%d", meta.Width, meta.Height, want.Bounds().Dx(), want.Bounds().Dy())
			}
		})
	}
}

func TestDecodeErrorPropagates(t *testing.T) {
	if _, _, err := New().Decode(nil); !errors.Is(err, domain.ErrTextureHeader) {
		t.Errorf("err = %v, want ErrTextureHeader", err)
	}
}
