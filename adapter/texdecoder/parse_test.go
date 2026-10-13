package texdecoder

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// texFixtures maps each staged .tex fixture to its expected pixel format.
// Formats and dims were validated empirically against the tex_png oracles
// on the full 5,433-file corpus during design; these seven cover DXT1
// (4-color + 3-color blocks), DXT3, DXT5 (both alpha tables), RGB24 and
// RGBA32, with and without trailing mip data.
var texFixtures = []struct {
	file   string
	format domain.TextureFormat
}{
	{"A_fire_solt_Sphere_D", domain.FmtDXT1}, // 4x4, one 4-color block, 16 B trailing mips
	{"CLI_Bottom_Side", domain.FmtDXT1},      // 128x128, 937/1024 blocks in 3-color mode
	{"A_Flower00_A", domain.FmtDXT3},         // 64x64, explicit alpha nibbles
	{"A_Miss", domain.FmtDXT5},               // 64x32, all blocks a0<=a1 (5-entry table)
	{"CH_E_Line", domain.FmtRGB24},           // 128x256, uncompressed 24-bit
	{"CH_E_Chain", domain.FmtRGBA32},         // 128x64, uncompressed 32-bit
	{"GUI_BackTile", domain.FmtRGBA32},       // 128x128, uncompressed 32-bit
}

func fixtureTex(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "tex", name+".tex"))
	if err != nil {
		t.Fatalf("missing fixture %s: %v", name, err)
	}
	return b
}

// oracleSize returns the committed reference PNG's dimensions.
func oracleSize(t *testing.T, name string) (int, int) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "tex", name+".png"))
	if err != nil {
		t.Fatalf("missing oracle %s: %v", name, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("oracle %s: %v", name, err)
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

// invert flips every byte of a whole file. A .tex only has its 128-byte
// header inverted, so a fully-inverted file presents a plain-DDS header to
// parse() and must be rejected.
func invert(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c ^ 0xFF
	}
	return out
}

// mutate flips one raw byte of the tex container.
func mutate(tex []byte, off int, val byte) []byte {
	out := append([]byte(nil), tex...)
	out[off] = val
	return out
}

// mutateAt sets a little-endian uint32 in the DECODED DDS view at off
// (writes the inverted bytes so parse() reads v after its XOR pass).
func mutateAt(tex []byte, off int, v uint32) []byte {
	out := append([]byte(nil), tex...)
	binary.LittleEndian.PutUint32(out[off:off+4], v^0xFFFFFFFF)
	return out
}

func TestParseFixtureMeta(t *testing.T) {
	for _, fx := range texFixtures {
		t.Run(fx.file, func(t *testing.T) {
			meta, pix, err := parse(fixtureTex(t, fx.file))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			w, h := oracleSize(t, fx.file)
			if meta.Width != w || meta.Height != h {
				t.Errorf("dims = %dx%d, want oracle %dx%d", meta.Width, meta.Height, w, h)
			}
			if meta.Format != fx.format {
				t.Errorf("format = %v, want %v", meta.Format, fx.format)
			}
			// mip0 is exactly the first expected-size bytes.
			var exp int
			bw, bh := (w+3)/4, (h+3)/4
			switch fx.format {
			case domain.FmtDXT1:
				exp = bw * bh * 8
			case domain.FmtDXT3, domain.FmtDXT5:
				exp = bw * bh * 16
			case domain.FmtRGB24:
				exp = w * h * 3
			case domain.FmtRGBA32:
				exp = w * h * 4
			}
			if len(pix) != exp {
				t.Errorf("mip0 len = %d, want %d", len(pix), exp)
			}
		})
	}
}

// TestParseBodyLeftUntouched pins the container contract observed in
// FantaTennis.exe (loader 0x5DBC00): the game inverts exactly the first
// 128 bytes (DDS header) of a .tex and uses the pixel body as stored.
// The parser must therefore return tex[128:] byte-for-byte.
func TestParseBodyLeftUntouched(t *testing.T) {
	for _, fx := range texFixtures {
		t.Run(fx.file, func(t *testing.T) {
			tex := fixtureTex(t, fx.file)
			_, pix, err := parse(tex)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pix, tex[128:128+len(pix)]) {
				t.Errorf("pixel body was transformed; parser must pass stored bytes through")
			}
		})
	}
}

func TestParseToleratesTrailingMips(t *testing.T) {
	// A_fire_solt_Sphere_D carries 16 trailing mip bytes after its 8-byte mip0.
	_, pix, err := parse(fixtureTex(t, "A_fire_solt_Sphere_D"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pix) != 8 {
		t.Errorf("mip0 = %d bytes, want 8 (trailing mips must be cut)", len(pix))
	}
}

func TestParseErrors(t *testing.T) {
	good := fixtureTex(t, "A_fire_solt_Sphere_D")
	cases := []struct {
		name string
		in   []byte
		want error
	}{
		{"nil", nil, domain.ErrTextureHeader},
		{"shorter than header", good[:100], domain.ErrTextureHeader},
		{"plain DDS without inversion", invert(good), domain.ErrTextureHeader},
		{"bad magic", mutate(good, 0, 0x00), domain.ErrTextureHeader},
		{"bad dwSize", mutateAt(good, 4, 125), domain.ErrTextureHeader},
		{"zero width", mutateAt(good, 16, 0), domain.ErrTextureHeader},
		{"truncated pixels", good[:130], domain.ErrTextureHeader}, // header + 2 < mip0 of 8
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := parse(c.in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
	// Unknown fourcc → format error, not header error.
	bad := append([]byte(nil), good...)
	bad[84] = 'X'
	if _, _, err := parse(bad); !errors.Is(err, domain.ErrTextureFormat) {
		t.Errorf("unknown fourcc err = %v, want ErrTextureFormat", err)
	}
	// Unsupported uncompressed masks (swap red/blue) → format error.
	raw := fixtureTex(t, "GUI_BackTile")
	badRaw := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint32(badRaw[92:96], 0xFF) // red mask := blue mask
	if _, _, err := parse(badRaw); !errors.Is(err, domain.ErrTextureFormat) {
		t.Errorf("bad masks err = %v, want ErrTextureFormat", err)
	}
}
