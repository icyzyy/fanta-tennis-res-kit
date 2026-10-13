package texdecoder

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// oracleRGBA loads the committed reference PNG as NRGBA pixels.
func oracleRGBA(t *testing.T, name string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "tex", name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return toNRGBA(img)
}

// toNRGBA normalizes any decoded image for byte-comparison. It must NOT go
// through color.Color's premultiplied RGBA() values: png RGBA oracles decode
// to *image.NRGBA, and premultiplication would corrupt true colors under
// alpha < 255 (8,006 bogus "differences" on A_Flower00_A before this fix).
func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		b := n.Bounds()
		out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		copy(out.Pix, n.Pix) // full-width, top-left origin → same layout
		return out
	}
	if n, ok := img.(*image.NRGBA64); ok && n.Rect.Min == (image.Point{}) {
		b := n.Bounds()
		out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		for i, j := 0, 0; j < len(n.Pix); i, j = i+4, j+2 {
			out.Pix[i] = n.Pix[j]
			out.Pix[i+1] = n.Pix[j+2]
			out.Pix[i+2] = n.Pix[j+4]
			out.Pix[i+3] = n.Pix[j+6]
		}
		return out
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(a >> 8)})
		}
	}
	return out
}

func cmpPixels(t *testing.T, name string, got *image.NRGBA) {
	t.Helper()
	want := oracleRGBA(t, name)
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		n := 0
		for i := range got.Pix {
			if got.Pix[i] != want.Pix[i] {
				n++
			}
		}
		t.Errorf("%s: %d differing bytes of %d", name, n, len(got.Pix))
	}
}

func TestDecodeDXTFixturesAgainstOracle(t *testing.T) {
	for _, fx := range texFixtures {
		if fx.format == domain.FmtRGB24 || fx.format == domain.FmtRGBA32 {
			continue // raw formats are Task 14's tests
		}
		t.Run(fx.file, func(t *testing.T) {
			meta, pix, err := parse(fixtureTex(t, fx.file))
			if err != nil {
				t.Fatal(err)
			}
			got := decodeDXT(pix, meta.Width, meta.Height, fx.format)
			cmpPixels(t, fx.file, got)
		})
	}
}

func TestDecodeDXT1AutoDetectOpaque(t *testing.T) {
	// 4 blocks (8×8): 3 blocks with c0 > c1, 1 block with c0 <= c1 using index 3.
	// Because c0 > c1 blocks outnumber the c0 <= c1 block (3 > 1), it is
	// auto-detected as an opaque texture and index 3 decodes as color 3 with Alpha=255.
	pix := make([]byte, 4*8)
	// Block 0 (0,0): c0=0x590a, c1=0x48ca (c0 > c1)
	pix[0], pix[1] = 0x0a, 0x59
	pix[2], pix[3] = 0xca, 0x48
	// Block 1 (1,0): c0=0x590a, c1=0x48ca (c0 > c1)
	pix[8], pix[9] = 0x0a, 0x59
	pix[10], pix[11] = 0xca, 0x48
	// Block 2 (0,1): c0=0x590a, c1=0x48ca (c0 > c1)
	pix[16], pix[17] = 0x0a, 0x59
	pix[18], pix[19] = 0xca, 0x48
	// Block 3 (1,1): c0=0x48ca, c1=0x590a (c0 <= c1) with index 3 on first pixel
	pix[24], pix[25] = 0xca, 0x48
	pix[26], pix[27] = 0x0a, 0x59
	pix[28] = 0x03 // pixel 0 has index 3

	img := decodeDXT(pix, 8, 8, domain.FmtDXT1)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if a := img.NRGBAAt(x, y).A; a != 255 {
				t.Fatalf("pixel (%d, %d) has alpha %d, want 255 (opaque)", x, y, a)
			}
		}
	}
}
