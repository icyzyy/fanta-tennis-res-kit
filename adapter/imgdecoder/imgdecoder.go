// Package imgdecoder decodes the image leftovers found inside FantaTennis
// .res archives — .tex, .dds, .bmp and .tga — to PNG payload + metadata,
// dispatching on the entry's file extension.
package imgdecoder

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"path/filepath"
	"strings"

	"github.com/icyzyy/fanta-tennis-res-kit/adapter/texdecoder"
	"github.com/icyzyy/fanta-tennis-res-kit/domain"
	usecase "github.com/icyzyy/fanta-tennis-res-kit/usecase"
	"golang.org/x/image/bmp"
)

// ddsHeaderSize is the fixed DDS header size (4-byte magic + 124-byte
// DDS_HEADER). The game's loader inverts exactly this many bytes of a
// .tex; a plain .dds carries its header un-inverted, so the .dds path
// synthesizes a .tex-shaped copy for texdecoder to invert back.
const ddsHeaderSize = 128

// Decoder dispatches image decoding on the entry extension.
type Decoder struct{}

var _ usecase.ImageDecoder = (*Decoder)(nil)

// New returns the image decoder.
func New() *Decoder { return &Decoder{} }

// Decode renders data as PNG bytes + metadata, choosing the decoder from
// name's extension (case-insensitive). The caller's slice is never mutated.
// Unknown extensions wrap domain.ErrNotTexture.
func (d *Decoder) Decode(name string, data []byte) (*domain.Texture, []byte, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".tex":
		return texdecoder.New().Decode(data)
	case ".dds":
		return decodeDDS(name, data)
	case ".bmp":
		return decodeBMP(name, data)
	case ".tga":
		return decodeTGA(name, data)
	default:
		return nil, nil, fmt.Errorf("imgdecoder %s: %w", name, domain.ErrNotTexture)
	}
}

// decodeDDS adapts a raw DDS file to the .tex container contract: copy the
// buffer, invert only its 128-byte header, and hand the .tex-shaped result
// to texdecoder (whose parse() inverts the header back to plain DDS).
func decodeDDS(name string, data []byte) (*domain.Texture, []byte, error) {
	tex := make([]byte, len(data))
	copy(tex, data)
	n := len(tex)
	if n > ddsHeaderSize {
		n = ddsHeaderSize
	}
	for i := 0; i < n; i++ {
		tex[i] ^= 0xFF
	}
	meta, pngBytes, err := texdecoder.New().Decode(tex)
	if err != nil {
		return nil, nil, fmt.Errorf("imgdecoder %s: %w", name, err)
	}
	return meta, pngBytes, nil
}

// decodeBMP decodes a classic BMP via golang.org/x/image/bmp.
func decodeBMP(name string, data []byte) (*domain.Texture, []byte, error) {
	img, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("imgdecoder %s: bmp: %w", name, err)
	}
	return encodePNG(name, img)
}

// encodePNG renders img as PNG and builds the metadata the preview needs.
func encodePNG(name string, img image.Image) (*domain.Texture, []byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, nil, fmt.Errorf("imgdecoder %s: encode png: %w", name, err)
	}
	b := img.Bounds()
	return &domain.Texture{Width: b.Dx(), Height: b.Dy(), Format: domain.FmtRGBA32}, buf.Bytes(), nil
}
