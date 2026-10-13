package texdecoder

import (
	"image"
	"image/color"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// decodeRaw decodes uncompressed mip-0 pixels. parse() has already verified
// the standard little-endian masks (R=0xff0000, G=0xff00, B=0xff[,
// A=0xff000000]), so the byte layout is B,G,R[,A].
func decodeRaw(pix []byte, w, h int, format domain.TextureFormat) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	bpp := 3
	if format == domain.FmtRGBA32 {
		bpp = 4
	}
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := byte(255)
			if bpp == 4 {
				a = pix[i+3]
			}
			img.SetNRGBA(x, y, color.NRGBA{R: pix[i+2], G: pix[i+1], B: pix[i], A: a})
			i += bpp
		}
	}
	return img
}
