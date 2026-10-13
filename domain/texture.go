package domain

// TextureFormat enumerates the pixel formats found in .tex files.
type TextureFormat int

const (
	FmtDXT1 TextureFormat = iota
	FmtDXT3
	FmtDXT5
	FmtRGB24
	FmtRGBA32
)

func (f TextureFormat) String() string {
	switch f {
	case FmtDXT1:
		return "DXT1"
	case FmtDXT3:
		return "DXT3"
	case FmtDXT5:
		return "DXT5"
	case FmtRGB24:
		return "RGB24"
	case FmtRGBA32:
		return "RGBA32"
	default:
		return "unknown"
	}
}

// Texture describes a decoded texture's geometry.
type Texture struct {
	Width  int
	Height int
	Format TextureFormat
}
