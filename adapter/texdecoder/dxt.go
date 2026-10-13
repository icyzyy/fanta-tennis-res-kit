package texdecoder

import (
	"encoding/binary"
	"image"
	"image/color"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// decodeDXT decodes mip-0 DXT1/DXT3/DXT5 pixels into an NRGBA image.
// The palette rules are the ones the game's assets were encoded with
// (validated byte-for-byte against the tex_png oracles):
//   - DXT1: order-based mode — c0 > c1 → 4 colors, else 3 colors + transparent black.
//   - DXT3/DXT5: always 4 colors; alpha from the block's alpha section.
func decodeDXT(pix []byte, w, h int, format domain.TextureFormat) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	bw, bh := (w+3)/4, (h+3)/4

	// In DXT1, auto-detect whether the texture is an opaque 4-color texture
	// (common across FantaTennis assets where c0 <= c1 was emitted by
	// 4-color compressors without punch-through alpha intent) or an
	// intentional 1-bit alpha texture. When c0 > c1 blocks outnumber
	// c0 <= c1 blocks that actually use index 3, the asset is treated
	// as opaque to avoid punching spurious transparent holes into solid tiles.
	dxt1Opaque := false
	if format == domain.FmtDXT1 {
		var c0gtC1, c0leC1With3 int
		for o := 0; o+8 <= len(pix); o += 8 {
			c0 := binary.LittleEndian.Uint16(pix[o : o+2])
			c1 := binary.LittleEndian.Uint16(pix[o+2 : o+4])
			if c0 > c1 {
				c0gtC1++
			} else {
				bits := binary.LittleEndian.Uint32(pix[o+4 : o+8])
				for i := 0; i < 16; i++ {
					if (bits>>(2*i))&3 == 3 {
						c0leC1With3++
						break
					}
				}
			}
		}
		dxt1Opaque = c0gtC1 > c0leC1With3
	}

	off := 0
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			var alphas [16]byte
			switch format {
			case domain.FmtDXT3:
				alphaBits := binary.LittleEndian.Uint64(pix[off : off+8])
				off += 8
				for i := 0; i < 16; i++ {
					alphas[i] = byte(((alphaBits >> (4 * i)) & 0xF) * 17)
				}
			case domain.FmtDXT5:
				a0, a1 := pix[off], pix[off+1]
				var alphaBits uint64
				for i := 0; i < 6; i++ {
					alphaBits |= uint64(pix[off+2+i]) << (8 * i)
				}
				off += 8
				var tbl [8]byte
				tbl[0], tbl[1] = a0, a1
				if a0 > a1 {
					for i := 1; i <= 6; i++ {
						tbl[i+1] = byte(((7-i)*int(a0) + i*int(a1)) / 7)
					}
				} else {
					for i := 1; i <= 4; i++ {
						tbl[i+1] = byte(((5-i)*int(a0) + i*int(a1)) / 5)
					}
					tbl[6], tbl[7] = 0, 255
				}
				for i := 0; i < 16; i++ {
					alphas[i] = tbl[(alphaBits>>(3*i))&7]
				}
			}

			c0 := binary.LittleEndian.Uint16(pix[off : off+2])
			c1 := binary.LittleEndian.Uint16(pix[off+2 : off+4])
			var colorBits uint32
			for i := 0; i < 4; i++ {
				colorBits |= uint32(pix[off+4+i]) << (8 * i)
			}
			off += 8

			p0, p1 := expand565(c0), expand565(c1)
			var pal [4]color.NRGBA
			pal[0], pal[1] = p0, p1
			fourColor := format != domain.FmtDXT1 || dxt1Opaque || c0 > c1
			if fourColor {
				pal[2] = mix(p0, p1, 2, 1, 3)
				pal[3] = mix(p0, p1, 1, 2, 3)
			} else {
				pal[2] = mix(p0, p1, 1, 1, 2)
				pal[3] = color.NRGBA{} // transparent black
			}

			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					X, Y := bx*4+px, by*4+py
					if X >= w || Y >= h {
						continue
					}
					idx := (colorBits >> (2 * (py*4 + px))) & 3
					c := pal[idx]
					if format == domain.FmtDXT1 {
						// palette alpha already correct (0 only for 3-color c3)
					} else {
						c.A = alphas[py*4+px]
					}
					img.SetNRGBA(X, Y, c)
				}
			}
		}
	}
	return img
}

// expand565 converts a 565 endpoint to NRGBA with bit replication.
func expand565(c uint16) color.NRGBA {
	r := uint8((c >> 11) & 0x1F)
	g := uint8((c >> 5) & 0x3F)
	b := uint8(c & 0x1F)
	return color.NRGBA{R: r<<3 | r>>2, G: g<<2 | g>>4, B: b<<3 | b>>2, A: 255}
}

// mix interpolates two endpoints with integer weights (wa*a+wb*b)/div.
func mix(a, b color.NRGBA, wa, wb, div int) color.NRGBA {
	return color.NRGBA{
		R: uint8((wa*int(a.R) + wb*int(b.R)) / div),
		G: uint8((wa*int(a.G) + wb*int(b.G)) / div),
		B: uint8((wa*int(a.B) + wb*int(b.B)) / div),
		A: 255,
	}
}
