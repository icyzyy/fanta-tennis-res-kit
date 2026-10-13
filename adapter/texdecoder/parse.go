package texdecoder

import (
	"encoding/binary"
	"fmt"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

const ddsHeaderSize = 128

// parse inverts a .tex container's 128-byte DDS header (the game's loader,
// FantaTennis.exe fcn.005DBC00, NOTs exactly 32 dwords = the header only;
// the pixel body is used as stored), validates the header, and returns
// metadata plus the mip-0 pixel slice. Trailing mip levels are tolerated
// and discarded.
func parse(tex []byte) (*domain.Texture, []byte, error) {
	if len(tex) < ddsHeaderSize {
		return nil, nil, fmt.Errorf("%w: %d bytes < %d", domain.ErrTextureHeader, len(tex), ddsHeaderSize)
	}
	dds := make([]byte, len(tex))
	for i := 0; i < ddsHeaderSize; i++ {
		dds[i] = tex[i] ^ 0xFF
	}
	copy(dds[ddsHeaderSize:], tex[ddsHeaderSize:])
	if string(dds[0:4]) != "DDS " || binary.LittleEndian.Uint32(dds[4:8]) != 124 {
		return nil, nil, fmt.Errorf("%w: not a DDS container (magic % x)", domain.ErrTextureHeader, dds[0:4])
	}
	height := int(binary.LittleEndian.Uint32(dds[12:16]))
	width := int(binary.LittleEndian.Uint32(dds[16:20]))
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, nil, fmt.Errorf("%w: implausible dimensions %dx%d", domain.ErrTextureHeader, width, height)
	}
	fourcc := string(dds[84:88])
	pfFlags := binary.LittleEndian.Uint32(dds[80:84])
	bits := binary.LittleEndian.Uint32(dds[88:92])
	blocks := ((width + 3) / 4) * ((height + 3) / 4)

	var (
		format domain.TextureFormat
		exp    int
	)
	switch fourcc {
	case "DXT1":
		format, exp = domain.FmtDXT1, blocks*8
	case "DXT3":
		format, exp = domain.FmtDXT3, blocks*16
	case "DXT5":
		format, exp = domain.FmtDXT5, blocks*16
	case "\x00\x00\x00\x00":
		red := binary.LittleEndian.Uint32(dds[92:96])
		green := binary.LittleEndian.Uint32(dds[96:100])
		blue := binary.LittleEndian.Uint32(dds[100:104])
		alpha := binary.LittleEndian.Uint32(dds[104:108])
		switch {
		case pfFlags&0x40 == 0:
			return nil, nil, fmt.Errorf("%w: uncompressed without DDPF_RGB flag 0x40 (flags %08x)",
				domain.ErrTextureFormat, pfFlags)
		case bits == 24 && red == 0xff0000 && green == 0xff00 && blue == 0xff:
			format, exp = domain.FmtRGB24, width*height*3
		case bits == 32 && red == 0xff0000 && green == 0xff00 && blue == 0xff && alpha == 0xff000000:
			format, exp = domain.FmtRGBA32, width*height*4
		default:
			return nil, nil, fmt.Errorf("%w: uncompressed bits=%d masks r=%08x g=%08x b=%08x a=%08x",
				domain.ErrTextureFormat, bits, red, green, blue, alpha)
		}
	default:
		return nil, nil, fmt.Errorf("%w: fourcc %q", domain.ErrTextureFormat, fourcc)
	}
	pix := dds[ddsHeaderSize:]
	if len(pix) < exp {
		return nil, nil, fmt.Errorf("%w: %d pixel bytes, want at least %d", domain.ErrTextureHeader, len(pix), exp)
	}
	return &domain.Texture{Width: width, Height: height, Format: format}, pix[:exp], nil
}
