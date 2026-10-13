package domain

import "testing"

func TestTextureFormatString(t *testing.T) {
	want := map[TextureFormat]string{
		FmtDXT1: "DXT1", FmtDXT3: "DXT3", FmtDXT5: "DXT5",
		FmtRGB24: "RGB24", FmtRGBA32: "RGBA32",
	}
	for f, s := range want {
		if f.String() != s {
			t.Errorf("Fmt(%d).String() = %q, want %q", f, f.String(), s)
		}
	}
}
