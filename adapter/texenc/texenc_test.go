package texenc

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/png"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/adapter/texdecoder"
	domain "github.com/icyzyy/fanta-tennis-res-kit/domain"
	usecase "github.com/icyzyy/fanta-tennis-res-kit/usecase"
)

// unxor strips the .tex container obfuscation to expose the raw DDS.
// Only the 128-byte header is inverted; the pixel body is stored raw.
func unxor(tex []byte) []byte {
	dds := make([]byte, len(tex))
	n := 128
	if len(dds) < n {
		n = len(dds)
	}
	for i := 0; i < n; i++ {
		dds[i] = tex[i] ^ 0xFF
	}
	copy(dds[n:], tex[n:])
	return dds
}

// solid builds a w×h NRGBA filled with one color.
func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// maxChannelErr returns the largest per-channel absolute difference between
// two decoded images of equal size.
func maxChannelErr(t *testing.T, got image.Image, want *image.NRGBA) int {
	t.Helper()
	b := got.Bounds()
	if b.Dx() != want.Rect.Dx() || b.Dy() != want.Rect.Dy() {
		t.Fatalf("decoded size %dx%d, want %dx%d", b.Dx(), b.Dy(), want.Rect.Dx(), want.Rect.Dy())
	}
	max := 0
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			// NRGBAModel: At().RGBA() is alpha-premultiplied, which would
			// skew comparisons of semi-transparent pixels.
			g := color.NRGBAModel.Convert(got.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			w := want.NRGBAAt(x, y)
			for _, d := range [4]int{
				abs(int(g.R) - int(w.R)),
				abs(int(g.G) - int(w.G)),
				abs(int(g.B) - int(w.B)),
				abs(int(g.A) - int(w.A)),
			} {
				if d > max {
					max = d
				}
			}
		}
	}
	return max
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// decodeWithOracle re-parses encoded .tex bytes through reskit's decoder —
// the same code path the app and the game's file shape are validated against.
func decodeWithOracle(t *testing.T, tex []byte) (*domain.Texture, image.Image) {
	t.Helper()
	meta, pngBytes, err := texdecoder.New().Decode(tex)
	if err != nil {
		t.Fatalf("reskit decode: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	return meta, img
}

func TestEncodeIsXORedDDS(t *testing.T) {
	tex, err := Encode(solid(8, 8, color.NRGBA{R: 200, G: 40, B: 40, A: 255}), domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dds := unxor(tex)
	if string(dds[0:4]) != "DDS " || binary.LittleEndian.Uint32(dds[4:8]) != 124 {
		t.Fatalf("not a DDS header: % x", dds[0:8])
	}
	if binary.LittleEndian.Uint32(dds[12:16]) != 8 || binary.LittleEndian.Uint32(dds[16:20]) != 8 {
		t.Fatalf("header dims %dx%d, want 8x8",
			binary.LittleEndian.Uint32(dds[16:20]), binary.LittleEndian.Uint32(dds[12:16]))
	}
	if got := string(dds[84:88]); got != "DXT1" {
		t.Fatalf("fourcc %q, want DXT1", got)
	}
	flags := binary.LittleEndian.Uint32(dds[8:12])
	if flags&0x20000 == 0 {
		t.Fatalf("flags 0x%x missing DDSD_MIPMAPCOUNT (0x20000)", flags)
	}
	caps1 := binary.LittleEndian.Uint32(dds[108:112])
	if caps1 != 0x401008 {
		t.Fatalf("caps1 0x%x, want 0x401008 (TEXTURE|MIPMAP|COMPLEX)", caps1)
	}
}

func TestEncodeDXT1SolidColorRoundTrip(t *testing.T) {
	want := solid(8, 8, color.NRGBA{R: 180, G: 90, B: 30, A: 255})
	tex, err := Encode(want, domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	meta, img := decodeWithOracle(t, tex)
	if meta.Width != 8 || meta.Height != 8 || meta.Format != domain.FmtDXT1 {
		t.Fatalf("meta %+v, want 8x8 DXT1", meta)
	}
	if errErr := maxChannelErr(t, img, want); errErr > 8 {
		t.Fatalf("channel error %d > 8 (565 quantization)", errErr)
	}
}

func TestEncodeDXT1GradientRoundTrip(t *testing.T) {
	want := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			want.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	tex, err := Encode(want, domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, img := decodeWithOracle(t, tex)
	if e := maxChannelErr(t, img, want); e > 48 {
		t.Fatalf("channel error %d > 48", e)
	}
}

func TestEncodeDXT1TransparencyKept(t *testing.T) {
	want := solid(4, 4, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	want.SetNRGBA(1, 1, color.NRGBA{A: 0})
	want.SetNRGBA(2, 3, color.NRGBA{A: 0})
	tex, err := Encode(want, domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, img := decodeWithOracle(t, tex)
	if _, _, _, a := img.At(1, 1).RGBA(); a != 0 {
		t.Fatalf("transparent pixel came back a=%d", a>>8)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0xffff {
		t.Fatalf("opaque pixel came back a=%d", a>>8)
	}
}

func TestEncodeDXT3AlphaRoundTrip(t *testing.T) {
	want := solid(4, 4, color.NRGBA{R: 10, G: 200, B: 90, A: 255})
	want.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 200, B: 90, A: 136})
	tex, err := Encode(want, domain.FmtDXT3)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	meta, img := decodeWithOracle(t, tex)
	if meta.Format != domain.FmtDXT3 {
		t.Fatalf("format %v, want DXT3", meta.Format)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); abs(int(a>>8)-136) > 8 {
		t.Fatalf("alpha 136 came back %d (4-bit quantization)", a>>8)
	}
}

func TestEncodeDXT5AlphaRoundTrip(t *testing.T) {
	want := solid(4, 4, color.NRGBA{R: 70, G: 70, B: 70, A: 255})
	want.SetNRGBA(3, 0, color.NRGBA{R: 70, G: 70, B: 70, A: 0})
	tex, err := Encode(want, domain.FmtDXT5)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	meta, img := decodeWithOracle(t, tex)
	if meta.Format != domain.FmtDXT5 {
		t.Fatalf("format %v, want DXT5", meta.Format)
	}
	if _, _, _, a := img.At(3, 0).RGBA(); a != 0 {
		t.Fatalf("alpha 0 came back %d", a>>8)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0xffff {
		t.Fatalf("alpha 255 came back %d", a>>8)
	}
}

func TestEncodeRGB24LayoutAndRoundTrip(t *testing.T) {
	want := solid(4, 2, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	tex, err := Encode(want, domain.FmtRGB24)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dds := unxor(tex)
	if bits := binary.LittleEndian.Uint32(dds[88:92]); bits != 24 {
		t.Fatalf("bitcount %d, want 24", bits)
	}
	pix := dds[128:]
	if pix[0] != 50 || pix[1] != 100 || pix[2] != 200 {
		t.Fatalf("first pixel BGR %v, want [50 100 200]", pix[0:3])
	}
	meta, img := decodeWithOracle(t, tex)
	if meta.Format != domain.FmtRGB24 || meta.Width != 4 || meta.Height != 2 {
		t.Fatalf("meta %+v, want 4x2 RGB24", meta)
	}
	if e := maxChannelErr(t, img, want); e != 0 {
		t.Fatalf("uncompressed round-trip error %d, want 0", e)
	}
}

func TestEncodeRGBA32RoundTripExact(t *testing.T) {
	want := solid(4, 4, color.NRGBA{R: 9, G: 88, B: 200, A: 77})
	tex, err := Encode(want, domain.FmtRGBA32)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	meta, img := decodeWithOracle(t, tex)
	if meta.Format != domain.FmtRGBA32 {
		t.Fatalf("format %v, want RGBA32", meta.Format)
	}
	if e := maxChannelErr(t, img, want); e != 0 {
		t.Fatalf("round-trip error %d, want 0", e)
	}
}

func TestEncodeWritesFullMipChain(t *testing.T) {
	// 8×8 DXT1: 8×8 (4 blocks) + 4×4 (1) + 2×2 (1) + 1×1 (1) = 7 blocks × 8 B.
	tex, err := Encode(solid(8, 8, color.NRGBA{R: 1, A: 255}), domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wantLen := 128 + 7*8
	if len(tex) != wantLen {
		t.Fatalf("len %d, want %d (header + 4 mip levels)", len(tex), wantLen)
	}
	dds := unxor(tex)
	if mc := binary.LittleEndian.Uint32(dds[28:32]); mc != 4 {
		t.Fatalf("mip count %d, want 4", mc)
	}
}

func TestEncodeUncompressedOddDimensions(t *testing.T) {
	want := solid(6, 4, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	tex, err := Encode(want, domain.FmtRGB24)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Full chain (Direct3D floor halving): 6×4 (24) + 3×2 (6) + 1×1 (1) = 31 px = 93 bytes.
	if wantLen := 128 + 93; len(tex) != wantLen {
		t.Fatalf("len %d, want %d", len(tex), wantLen)
	}
	if _, img := decodeWithOracle(t, tex); maxChannelErr(t, img, want) != 0 {
		t.Fatal("odd-dim round-trip not exact")
	}
}

func TestEncodeContrastingColorsPreserved(t *testing.T) {
	// A 4×4 block with two contrasting colors (pure red and pure green).
	// Under naive component-wise min/max, endpoints become yellow and black.
	// Euclidean max-distance endpoint selection preserves red and green.
	want := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if (x+y)%2 == 0 {
				want.SetNRGBA(x, y, red)
			} else {
				want.SetNRGBA(x, y, green)
			}
		}
	}
	tex, err := Encode(want, domain.FmtDXT1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, img := decodeWithOracle(t, tex)
	// Both colors should be preserved within 565 quantization error (≤ 8).
	if errErr := maxChannelErr(t, img, want); errErr > 8 {
		t.Fatalf("contrasting color error %d > 8", errErr)
	}
}

func TestTextureEncoderPort(t *testing.T) {
	var enc usecase.TextureEncoder = New()
	tex, err := enc.Encode(solid(4, 4, color.NRGBA{R: 120, A: 255}), domain.FmtDXT1)
	if err != nil {
		t.Fatalf("enc.Encode: %v", err)
	}
	if len(tex) < 128 {
		t.Fatalf("len %d < 128", len(tex))
	}
}

func TestEncodeRejectsNilImage(t *testing.T) {
	if _, err := Encode(nil, domain.FmtDXT1); err == nil {
		t.Fatal("nil image accepted")
	}
}
