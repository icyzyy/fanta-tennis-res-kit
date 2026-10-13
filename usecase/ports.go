package usecase

import (
	"image"
	"time"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// ArchiveRepo reads and atomically rewrites .res archives.
// Implementations must be safe for concurrent readers and serialize
// Replace per archive.
type ArchiveRepo interface {
	List(archive string) ([]domain.Entry, error)
	Read(archive, entry string) ([]byte, error)
	// Replace writes data for entry in place: same-dir tmp → fsync → verify →
	// rename. The original stays intact on any failure.
	Replace(archive, entry string, data []byte) error
	// SetBackup toggles copying the original to <archive>.bak before the
	// first write of a session (default on; set by the composition root).
	SetBackup(enabled bool)
}

// SetCipher encrypts/decrypts .set container bytes.
type SetCipher interface {
	Encrypt(plain []byte) ([]byte, error)
	Decrypt(file []byte) ([]byte, error)
}

// TextureDecoder decodes .tex bytes to PNG payload + metadata.
type TextureDecoder interface {
	Decode(tex []byte) (*domain.Texture, []byte, error)
}

// ImageDecoder decodes any supported image entry (.tex/.dds/.bmp/.tga)
// to PNG payload + metadata, dispatching on the entry name.
type ImageDecoder interface {
	Decode(name string, data []byte) (*domain.Texture, []byte, error)
}

// TextureEncoder encodes an image to .tex bytes.
type TextureEncoder interface {
	Encode(src image.Image, format domain.TextureFormat) ([]byte, error)
}

// FileSystem is the disk surface used by interactors.
type FileSystem interface {
	WalkRes(root string) ([]string, error)
	Stat(path string) (size int64, modTime time.Time, err error)
	WriteFile(path string, data []byte) error // creates parent dirs
}
