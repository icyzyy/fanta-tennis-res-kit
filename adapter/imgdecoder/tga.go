package imgdecoder

import (
	"fmt"
	"image"
	"image/color"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// TGA format facts (hand-derived; conventions cross-checked once against
// Pillow's independent TGA decoder):
//   - 18-byte header, all multi-byte fields little-endian
//   - image types 1/9 color-mapped, 2/10 truecolor, 3/11 grayscale;
//     9-11 are RLE variants (0x80 packet flag, count = (b&0x7F)+1)
//   - truecolor stores components BLUE first (BGR / BGRA byte order)
//   - 15/16-bit samples are X1R5G5B5 (R at bits 14-10) with attribute
//     polarity 1 = transparent, 0 = opaque (15-bit entries have no
//     attribute bit and decode opaque)
//   - color-map entries are 15/16 (2 bytes), 24 (BGR) or 32 (BGRA);
//     indices address cmapFirst+cmapLen, anything else is an error
//   - descriptor bit 0x20 = first stored row is TOP; otherwise the
//     image starts at the BOTTOM (first stored row is flipped to the end)
//   - descriptor bits 0xC0 (interleaving) are rejected as unsupported

// decodeTGA decodes a TGA file to PNG + metadata.
func decodeTGA(name string, data []byte) (*domain.Texture, []byte, error) {
	img, err := parseTGA(data)
	if err != nil {
		return nil, nil, fmt.Errorf("imgdecoder %s: tga: %w", name, err)
	}
	return encodePNG(name, img)
}

// parseTGA reads one complete TGA image. Every read is bounds-checked;
// malformed input yields an error, never a panic.
func parseTGA(data []byte) (*image.NRGBA, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("%d bytes < 18-byte header", len(data))
	}
	idLen := int(data[0])
	cmapType := data[1]
	imgType := data[2]
	cmapFirst := int(le16(data[3:5]))
	cmapLen := int(le16(data[5:7]))
	cmapEntrySize := int(data[7])
	w := int(le16(data[12:14]))
	h := int(le16(data[14:16]))
	depth := int(data[16])
	desc := data[17]

	colormapped := imgType == 1 || imgType == 9
	switch imgType {
	case 1, 2, 3, 9, 10, 11:
	default:
		return nil, fmt.Errorf("unsupported image type %d", imgType)
	}
	if colormapped && cmapType != 1 {
		return nil, fmt.Errorf("color-mapped image without color map (cmap type %d)", cmapType)
	}
	if cmapType > 1 {
		return nil, fmt.Errorf("unsupported color map type %d", cmapType)
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid dimensions %dx%d", w, h)
	}
	if desc&0xC0 != 0 {
		return nil, fmt.Errorf("unsupported interleaving descriptor 0x%02x", desc)
	}

	// Bytes stored per pixel or per color-map index.
	spp := 1
	switch {
	case colormapped:
		if depth != 8 && depth != 16 {
			return nil, fmt.Errorf("color-mapped index depth %d", depth)
		}
		spp = (depth + 7) / 8
	case imgType == 2 || imgType == 10:
		switch depth {
		case 15, 16:
			spp = 2
		case 24:
			spp = 3
		case 32:
			spp = 4
		default:
			return nil, fmt.Errorf("truecolor depth %d", depth)
		}
	default: // 3, 11 — grayscale
		if depth != 8 && depth != 16 {
			return nil, fmt.Errorf("grayscale depth %d", depth)
		}
		spp = (depth + 7) / 8
	}

	pos := 18 + idLen
	if pos > len(data) {
		return nil, fmt.Errorf("truncated id field")
	}

	// Color map (parsed for any file that declares one, required for 1/9).
	var cmap [][4]byte
	if cmapType != 0 {
		switch cmapEntrySize {
		case 15, 16, 24, 32:
		default:
			return nil, fmt.Errorf("color map entry size %d", cmapEntrySize)
		}
		entryBytes := (cmapEntrySize + 7) / 8
		need := cmapLen * entryBytes
		if pos+need > len(data) {
			return nil, fmt.Errorf("truncated color map: need %d bytes at %d, have %d", need, pos, len(data)-pos)
		}
		cmap = make([][4]byte, cmapLen)
		for i := 0; i < cmapLen; i++ {
			cmap[i] = decodeCmapEntry(data[pos:pos+entryBytes], cmapEntrySize)
			pos += entryBytes
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	total := w * h
	topOrigin := desc&0x20 != 0

	// row returns the destination y for the n-th stored scanline.
	row := func(n int) int {
		if topOrigin {
			return n
		}
		return h - 1 - n
	}

	decodeOne := func(src []byte) (color.NRGBA, error) {
		if imgType == 1 || imgType == 9 {
			idx := int(src[0])
			if depth == 16 {
				idx = int(le16(src))
			}
			entry := idx - cmapFirst
			if entry < 0 || entry >= len(cmap) {
				return color.NRGBA{}, fmt.Errorf("color map index %d out of range [%d,%d)", idx, cmapFirst, cmapFirst+len(cmap))
			}
			e := cmap[entry]
			return color.NRGBA{R: e[0], G: e[1], B: e[2], A: e[3]}, nil
		}
		r, g, b, a := decodeSample(src, imgType, depth)
		return color.NRGBA{R: r, G: g, B: b, A: a}, nil
	}

	switch {
	case imgType >= 9: // RLE
		decoded := 0
		for decoded < total {
			if pos >= len(data) {
				return nil, fmt.Errorf("truncated RLE data: %d of %d pixels", decoded, total)
			}
			b := data[pos]
			pos++
			count := int(b&0x7F) + 1
			if decoded+count > total {
				return nil, fmt.Errorf("RLE packet overruns image: %d+%d > %d", decoded, count, total)
			}
			if b&0x80 != 0 { // RLE packet: one sample repeated
				if pos+spp > len(data) {
					return nil, fmt.Errorf("truncated RLE sample")
				}
				px, err := decodeOne(data[pos : pos+spp])
				if err != nil {
					return nil, err
				}
				pos += spp
				for i := 0; i < count; i++ {
					x := (decoded + i) % w
					y := row((decoded + i) / w)
					img.SetNRGBA(x, y, px)
				}
			} else { // raw packet
				for i := 0; i < count; i++ {
					if pos+spp > len(data) {
						return nil, fmt.Errorf("truncated raw packet at pixel %d", decoded+i)
					}
					px, err := decodeOne(data[pos : pos+spp])
					if err != nil {
						return nil, err
					}
					pos += spp
					x := (decoded + i) % w
					y := row((decoded + i) / w)
					img.SetNRGBA(x, y, px)
				}
			}
			decoded += count
		}
	default: // uncompressed
		need := total * spp
		if pos+need > len(data) {
			return nil, fmt.Errorf("truncated image data: need %d bytes at %d, have %d", need, pos, len(data)-pos)
		}
		for n := 0; n < total; n++ {
			px, err := decodeOne(data[pos : pos+spp])
			if err != nil {
				return nil, err
			}
			pos += spp
			x := n % w
			y := row(n / w)
			img.SetNRGBA(x, y, px)
		}
	}
	return img, nil
}

// decodeCmapEntry converts one color-map entry to RGBA.
func decodeCmapEntry(b []byte, entrySize int) [4]byte {
	switch entrySize {
	case 15, 16:
		return sample555(le16(b))
	case 24:
		return [4]byte{b[2], b[1], b[0], 255} // BGR
	default: // 32
		return [4]byte{b[2], b[1], b[0], b[3]} // BGRA
	}
}

// decodeSample converts one stored pixel to RGBA for the given image type
// and depth. src has exactly the bytes the depth prescribes. Color-mapped
// types are handled by the caller (their index range is validated there).
func decodeSample(src []byte, imgType byte, depth int) (uint8, uint8, uint8, uint8) {
	switch {
	case imgType == 2 || imgType == 10:
		switch depth {
		case 15, 16:
			c := sample555(le16(src))
			return c[0], c[1], c[2], c[3]
		case 24:
			return src[2], src[1], src[0], 255 // BGR
		default: // 32
			return src[2], src[1], src[0], src[3] // BGRA
		}
	default: // 3, 11 grayscale
		if depth == 8 {
			return src[0], src[0], src[0], 255
		}
		g := uint8(le16(src) >> 8)
		return g, g, g, 255
	}
}

// sample555 converts an X1R5G5B5 value to RGBA. Attribute polarity follows
// Pillow's independent decoder: attribute bit SET means transparent.
func sample555(v uint16) [4]byte {
	a := uint8(255)
	if v&0x8000 != 0 {
		a = 0
	}
	return [4]byte{scale5(uint8(v>>10) & 31), scale5(uint8(v>>5) & 31), scale5(uint8(v) & 31), a}
}

// scale5 expands a 5-bit channel to 8 bits by bit replication (31 → 255).
func scale5(v uint8) uint8 { return v<<3 | v>>2 }

// le16 reads a little-endian uint16.
func le16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
