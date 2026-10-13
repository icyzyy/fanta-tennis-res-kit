package texdecoder

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// Decoder implements usecase.TextureDecoder for .tex files.
type Decoder struct{}

// New returns the texture decoder.
func New() *Decoder { return &Decoder{} }

// Decode inverts and parses a .tex file, decodes mip-0 to RGBA and encodes
// the result as PNG bytes (what the frontend preview renders).
func (d *Decoder) Decode(tex []byte) (*domain.Texture, []byte, error) {
	meta, pix, err := parse(tex)
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, decodeImage(meta, pix)); err != nil {
		return nil, nil, fmt.Errorf("texdecoder: encode png: %w", err)
	}
	return meta, buf.Bytes(), nil
}

// decodeImage dispatches on the parsed format.
func decodeImage(meta *domain.Texture, pix []byte) *image.NRGBA {
	switch meta.Format {
	case domain.FmtDXT1, domain.FmtDXT3, domain.FmtDXT5:
		return decodeDXT(pix, meta.Width, meta.Height, meta.Format)
	default:
		return decodeRaw(pix, meta.Width, meta.Height, meta.Format)
	}
}
