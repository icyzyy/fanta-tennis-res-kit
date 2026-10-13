package imgdecoder

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
	"golang.org/x/image/bmp"
)

// texFixture loads a committed .tex fixture and derives the equivalent raw
// .dds bytes (the .tex container is a DDS whose 128-byte header is inverted
// — see 5eef44f). The derived buffer must carry the plain DDS magic, which
// is the independent ground check on that contract.
func texFixture(t *testing.T) (tex, dds []byte) {
	t.Helper()
	tex, err := os.ReadFile(filepath.Join("..", "..", "testdata", "tex", "A_fire_solt_Sphere_D.tex"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dds = append([]byte(nil), tex...)
	for i := 0; i < 128 && i < len(dds); i++ {
		dds[i] ^= 0xFF
	}
	if string(dds[:4]) != "DDS " {
		t.Fatalf("derived dds magic = %q, want \"DDS \"", dds[:4])
	}
	return tex, dds
}

// TestDecodeTexAndDDSProduceIdenticalOutput catches a dispatch or
// container-contract break: the same asset named .tex and .dds must yield
// byte-identical PNG and metadata, extensions are case-insensitive, and
// neither path may mutate the caller's slice.
func TestDecodeTexAndDDSProduceIdenticalOutput(t *testing.T) {
	tex, dds := texFixture(t)
	texOrig := append([]byte(nil), tex...)
	ddsOrig := append([]byte(nil), dds...)

	d := New()
	metaTex, pngTex, err := d.Decode("A_fire_solt_Sphere_D.tex", tex)
	if err != nil {
		t.Fatalf(".tex decode: %v", err)
	}
	metaDDS, pngDDS, err := d.Decode("A_fire_solt_Sphere_D.dds", dds)
	if err != nil {
		t.Fatalf(".dds decode: %v", err)
	}
	if *metaTex != *metaDDS {
		t.Errorf("meta differs: .tex=%+v .dds=%+v", metaTex, metaDDS)
	}
	if !bytes.Equal(pngTex, pngDDS) {
		t.Errorf(".tex and .dds PNGs differ (%d vs %d bytes)", len(pngTex), len(pngDDS))
	}

	_, pngUpper, err := d.Decode("UPPER.TEX", tex)
	if err != nil {
		t.Fatalf("uppercase .TEX decode: %v", err)
	}
	if !bytes.Equal(pngUpper, pngTex) {
		t.Error("uppercase .TEX extension produced different PNG")
	}

	if !bytes.Equal(tex, texOrig) {
		t.Error(".tex path mutated the caller's slice")
	}
	if !bytes.Equal(dds, ddsOrig) {
		t.Error(".dds path mutated the caller's slice")
	}
}

// TestDecodeBMP catches a broken .bmp path or metadata: a hand-built image
// encoded with x/image/bmp must come back with bounds metadata and pixels
// equal to the literals we put in.
func TestDecodeBMP(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	want := [][3]uint8{
		{255, 0, 0}, {0, 255, 0}, {0, 0, 255},
		{10, 20, 30}, {255, 255, 0}, {7, 8, 9},
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			c := want[y*3+x]
			src.SetNRGBA(x, y, color.NRGBA{R: c[0], G: c[1], B: c[2], A: 255})
		}
	}
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, src); err != nil {
		t.Fatalf("bmp.Encode: %v", err)
	}

	meta, pngBytes, err := New().Decode("cursor.bmp", buf.Bytes())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if meta.Width != 3 || meta.Height != 2 || meta.Format != domain.FmtRGBA32 {
		t.Errorf("meta = %+v, want {3 2 RGBA32}", meta)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode returned png: %v", err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			w := want[y*3+x]
			wantc := color.NRGBA{R: w[0], G: w[1], B: w[2], A: 255}
			if got != wantc {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, wantc)
			}
		}
	}
}

// TestDecodeUnsupportedExtension catches a dispatch default-branch break:
// anything that is not a known image extension must wrap ErrNotTexture.
func TestDecodeUnsupportedExtension(t *testing.T) {
	_, _, err := New().Decode("LevelExp_Pet.set", []byte{0x01, 0x02})
	if !errors.Is(err, domain.ErrNotTexture) {
		t.Errorf("err = %v, want wrapped ErrNotTexture", err)
	}
}
