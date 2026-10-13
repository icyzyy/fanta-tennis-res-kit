package imgdecoder

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// tga builds an 18-byte TGA header. Color-map type is derived from cmapLen.
// Conventions below are hand-derived from the TGA layout and cross-checked
// once against Pillow's independent TGA decoder (BGR component order,
// bottom-origin default, top-origin bit 0x20, X1R5G5B5 with attribute
// 1=transparent).
func tga(imgType byte, cmapFirst, cmapLen uint16, cmapEntry byte, w, h uint16, depth, desc byte) []byte {
	b := make([]byte, 18)
	if cmapLen > 0 {
		b[1] = 1
	}
	b[2] = imgType
	b[3] = byte(cmapFirst)
	b[4] = byte(cmapFirst >> 8)
	b[5] = byte(cmapLen)
	b[6] = byte(cmapLen >> 8)
	b[7] = cmapEntry
	b[12] = byte(w)
	b[13] = byte(w >> 8)
	b[14] = byte(h)
	b[15] = byte(h >> 8)
	b[16] = depth
	b[17] = desc
	return b
}

func nrgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }

// decodeTGAInput runs data through the public Decode and returns the PNG
// pixels row-major plus the metadata.
func decodeTGAInput(t *testing.T, data []byte) (*domain.Texture, []color.NRGBA) {
	t.Helper()
	meta, pngBytes, err := New().Decode("img.tga", data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode returned png: %v", err)
	}
	b := img.Bounds()
	pix := make([]color.NRGBA, 0, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			pix = append(pix, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
		}
	}
	return meta, pix
}

func wantPixels(t *testing.T, got, want []color.NRGBA) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d pixels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pixel %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestDecodeTGA24BottomOrigin catches broken component order or origin
// handling: type 2, 24-bit BGR, descriptor bit 0x00 = first stored row is
// the BOTTOM row.
func TestDecodeTGA24BottomOrigin(t *testing.T) {
	data := append(tga(2, 0, 0, 0, 2, 2, 24, 0x00),
		// stored bottom row first: blue, white; then top row: red, green
		255, 0, 0, 255, 255, 255, 0, 0, 255, 0, 255, 0)
	meta, pix := decodeTGAInput(t, data)
	if meta.Width != 2 || meta.Height != 2 || meta.Format != domain.FmtRGBA32 {
		t.Errorf("meta = %+v, want {2 2 RGBA32}", meta)
	}
	wantPixels(t, pix, []color.NRGBA{
		nrgba(255, 0, 0, 255), nrgba(0, 255, 0, 255),
		nrgba(0, 0, 255, 255), nrgba(255, 255, 255, 255),
	})
}

// TestDecodeTGAColorMap32TopOrigin catches broken colormap lookup or
// origin handling: type 1, 32-bit BGRA entries, colormap starting at
// index 5 (cmapFirst), descriptor bit 0x20 = first stored row is TOP.
func TestDecodeTGAColorMap32TopOrigin(t *testing.T) {
	body := []byte{
		// colormap slots for indices 5..7: red/opaque, green/opaque,
		// (R10,G20,B30)/A40 — stored BGRA
		0, 0, 255, 255, 0, 255, 0, 255, 30, 20, 10, 40,
		// top row: 5,6,7 ; bottom row: 7,6,5
		5, 6, 7, 7, 6, 5,
	}
	data := append(tga(1, 5, 3, 32, 3, 2, 8, 0x20), body...)
	meta, pix := decodeTGAInput(t, data)
	if meta.Width != 3 || meta.Height != 2 || meta.Format != domain.FmtRGBA32 {
		t.Errorf("meta = %+v, want {3 2 RGBA32}", meta)
	}
	wantPixels(t, pix, []color.NRGBA{
		nrgba(255, 0, 0, 255), nrgba(0, 255, 0, 255), nrgba(10, 20, 30, 40),
		nrgba(10, 20, 30, 40), nrgba(0, 255, 0, 255), nrgba(255, 0, 0, 255),
	})
}

// TestDecodeTGARLEPacket catches broken RLE packet parsing: type 10 with
// one RLE packet (header 0x81, count 2, one BGR pixel) followed by one raw
// packet (header 0x01, count 2, two BGR pixels).
func TestDecodeTGARLEPacket(t *testing.T) {
	data := append(tga(10, 0, 0, 0, 4, 1, 24, 0x20),
		0x81, 1, 2, 3,
		0x01, 10, 20, 30, 0x40, 0x80, 0xC0)
	_, pix := decodeTGAInput(t, data)
	wantPixels(t, pix, []color.NRGBA{
		nrgba(3, 2, 1, 255), nrgba(3, 2, 1, 255),
		nrgba(30, 20, 10, 255), nrgba(192, 128, 64, 255),
	})
}

// TestDecodeTGAGrayscale catches the gray conversion branches: type 3
// (uncompressed 8-bit) and type 11 (RLE 8-bit).
func TestDecodeTGAGrayscale(t *testing.T) {
	t.Run("type3", func(t *testing.T) {
		data := append(tga(3, 0, 0, 0, 2, 1, 8, 0x20), 0x00, 0x80)
		_, pix := decodeTGAInput(t, data)
		wantPixels(t, pix, []color.NRGBA{nrgba(0, 0, 0, 255), nrgba(128, 128, 128, 255)})
	})
	t.Run("type11-RLE", func(t *testing.T) {
		data := append(tga(11, 0, 0, 0, 4, 1, 8, 0x20), 0x83, 0x40)
		_, pix := decodeTGAInput(t, data)
		wantPixels(t, pix, []color.NRGBA{
			nrgba(64, 64, 64, 255), nrgba(64, 64, 64, 255),
			nrgba(64, 64, 64, 255), nrgba(64, 64, 64, 255),
		})
	})
}

// TestDecodeTGAEncodings pins the 15/16-bit pixel and colormap-entry
// encodings (X1R5G5B5, R at bits 14-10; attribute bit 1 = transparent —
// cross-checked against Pillow). Expectations are hand-computed from the
// 5-bit channel values (31 → 255).
func TestDecodeTGAEncodings(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []color.NRGBA
	}{
		{
			"type2 depth16",
			append(tga(2, 0, 0, 0, 3, 1, 16, 0x20),
				0x00, 0x7C, // 0x7C00: R=31 → red, bit15=0 → opaque
				0xE0, 0x03, // 0x03E0: G=31 → green, opaque
				0x00, 0x80), // 0x8000: bit15=1 → transparent black
			[]color.NRGBA{nrgba(255, 0, 0, 255), nrgba(0, 255, 0, 255), nrgba(0, 0, 0, 0)},
		},
		{
			"type2 depth15",
			append(tga(2, 0, 0, 0, 1, 1, 15, 0x20), 0x00, 0x7C),
			[]color.NRGBA{nrgba(255, 0, 0, 255)},
		},
		{
			"type1 cmap entry16",
			append(tga(1, 0, 2, 16, 2, 1, 8, 0x20),
				0x00, 0x7C, // entry0: R=31, bit15=0 → red opaque
				0x00, 0xFC, // entry1: R=31, bit15=1 → red transparent
				0x00, 0x01), // indices 0,1
			[]color.NRGBA{nrgba(255, 0, 0, 255), nrgba(255, 0, 0, 0)},
		},
		{
			"type1 cmap entry15",
			append(tga(1, 0, 1, 15, 1, 1, 8, 0x20),
				0x00, 0x7C, // entry: R=31 → red opaque
				0x00),
			[]color.NRGBA{nrgba(255, 0, 0, 255)},
		},
		{
			"type1 cmap entry24",
			append(tga(1, 0, 1, 24, 1, 1, 8, 0x20),
				30, 20, 10, // BGR → (10,20,30)
				0x00),
			[]color.NRGBA{nrgba(10, 20, 30, 255)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, pix := decodeTGAInput(t, tt.data)
			wantPixels(t, pix, tt.want)
		})
	}
}

// TestDecodeTGAStrictErrors catches missing bounds validation: truncated
// input, out-of-range colormap indices and unsupported descriptors must
// return errors, never panic or mis-decode.
func TestDecodeTGAStrictErrors(t *testing.T) {
	full := append(tga(2, 0, 0, 0, 2, 2, 24, 0x00),
		255, 0, 0, 255, 255, 255, 0, 0, 255, 0, 255, 0)
	tests := []struct {
		name string
		data []byte
	}{
		{"truncated image data", full[:len(full)-3]},
		{"short header", full[:10]},
		{"empty", nil},
		{
			"colormap index out of range",
			append(tga(1, 0, 2, 32, 1, 1, 8, 0x20),
				0, 0, 255, 255, 0, 255, 0, 255,
				5), // index 5, only slots 0..1 exist
		},
		{"interleaved descriptor", append(tga(2, 0, 0, 0, 1, 1, 24, 0x40), 1, 2, 3)},
		{"unsupported image type", append(tga(4, 0, 0, 0, 1, 1, 24, 0x20), 1, 2, 3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := New().Decode("img.tga", tt.data)
			if err == nil {
				t.Error("err = nil, want error")
			}
		})
	}
}
