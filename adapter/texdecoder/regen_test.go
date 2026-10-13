package texdecoder

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegenOracles rewrites testdata/tex/*.png from the current decoder.
// It is skipped unless REGEN_ORACLES=1. The oracles are regression
// snapshots of the decode pipeline; the container contract they encode
// (128-byte header inverted, body raw) is grounded in the game binary,
// FantaTennis.exe loader fcn.005DBC00.
func TestRegenOracles(t *testing.T) {
	if os.Getenv("REGEN_ORACLES") != "1" {
		t.Skip("set REGEN_ORACLES=1 to regenerate oracle PNGs")
	}
	for _, fx := range texFixtures {
		meta, pngBytes, err := New().Decode(fixtureTex(t, fx.file))
		if err != nil {
			t.Fatalf("%s: Decode: %v", fx.file, err)
		}
		if meta == nil {
			t.Fatalf("%s: nil meta", fx.file)
		}
		path := filepath.Join("..", "..", "testdata", "tex", fx.file+".png")
		if err := os.WriteFile(path, pngBytes, 0o644); err != nil {
			t.Fatalf("%s: write: %v", fx.file, err)
		}
		t.Logf("%s: regenerated (%d bytes)", fx.file, len(pngBytes))
	}
}
