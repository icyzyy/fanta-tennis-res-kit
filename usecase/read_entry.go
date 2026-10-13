package usecase

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// ReadEntry returns an entry's decoded content:
// .set → decrypted XML, .tex → PNG + metadata, other → text or raw bytes.
type ReadEntry struct {
	repo   ArchiveRepo
	cipher SetCipher
	tex    TextureDecoder
}

// NewReadEntry wires the interactor.
func NewReadEntry(repo ArchiveRepo, cipher SetCipher, tex TextureDecoder) *ReadEntry {
	return &ReadEntry{repo: repo, cipher: cipher, tex: tex}
}

// Do reads and decodes one entry.
func (x *ReadEntry) Do(ctx context.Context, archive, entry string) (*Content, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("read_entry: %w", err)
	}
	data, err := x.repo.Read(archive, entry)
	if err != nil {
		return nil, fmt.Errorf("read_entry %s/%s: %w", archive, entry, err)
	}
	switch domain.KindOf(entry) {
	case domain.KindSet:
		plain, err := x.cipher.Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("read_entry %s/%s: decrypt: %w", archive, entry, err)
		}
		return &Content{Kind: ContentXML, Text: string(plain)}, nil
	case domain.KindTex:
		meta, png, err := x.tex.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("read_entry %s/%s: texture: %w", archive, entry, err)
		}
		return &Content{Kind: ContentPNG, Data: png, Texture: meta}, nil
	default:
		if isText(data) {
			return &Content{Kind: ContentText, Text: string(data)}, nil
		}
		return &Content{Kind: ContentBinary, Data: data}, nil
	}
}

// isText treats empty/valid-UTF-8-without-NUL content as text.
func isText(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return utf8.Valid(b)
}
