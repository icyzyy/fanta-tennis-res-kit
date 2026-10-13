package usecase

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// SaveEntry validates, encrypts and atomically repacks one .set entry, then
// verifies the round-trip by re-reading it.
type SaveEntry struct {
	repo   ArchiveRepo
	cipher SetCipher
}

// NewSaveEntry wires the interactor.
func NewSaveEntry(repo ArchiveRepo, cipher SetCipher) *SaveEntry {
	return &SaveEntry{repo: repo, cipher: cipher}
}

// Do performs the save. On any failure the original archive is untouched.
func (x *SaveEntry) Do(ctx context.Context, req SaveRequest) (*SaveResult, error) {
	const op = "save_entry"
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if domain.KindOf(req.Entry) != domain.KindSet {
		return nil, fmt.Errorf("%s %s/%s: %w", op, req.Archive, req.Entry, domain.ErrNotEditable)
	}
	if err := validateXML(req.Text); err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w", op, req.Archive, req.Entry, err)
	}
	enc, err := x.cipher.Encrypt([]byte(req.Text))
	if err != nil {
		return nil, fmt.Errorf("%s %s/%s: encrypt: %w", op, req.Archive, req.Entry, err)
	}
	if err := x.repo.Replace(req.Archive, req.Entry, enc); err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w", op, req.Archive, req.Entry, err)
	}
	// Round-trip verify: re-read, decrypt, compare.
	got, err := x.repo.Read(req.Archive, req.Entry)
	if err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w: reread: %v", op, req.Archive, req.Entry, domain.ErrVerifyFailed, err)
	}
	plain, err := x.cipher.Decrypt(got)
	if err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w: redecrypt: %v", op, req.Archive, req.Entry, domain.ErrVerifyFailed, err)
	}
	if string(plain) != req.Text {
		return nil, fmt.Errorf("%s %s/%s: %w: content differs after write", op, req.Archive, req.Entry, domain.ErrVerifyFailed)
	}
	return &SaveResult{OK: true, Revision: crc32.ChecksumIEEE([]byte(req.Text))}, nil
}

// validateXML enforces well-formedness stricter than encoding/xml alone:
// exactly one root element, nothing but whitespace outside it.
func validateXML(s string) error {
	dec := xml.NewDecoder(strings.NewReader(s))
	depth, roots := 0, 0
	closed := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var se *xml.SyntaxError
			if errors.As(err, &se) {
				return fmt.Errorf("%w: line %d: %s", domain.ErrInvalidXML, se.Line, se.Msg)
			}
			return fmt.Errorf("%w: %v", domain.ErrInvalidXML, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if closed {
					return fmt.Errorf("%w: content after root element <%s>", domain.ErrInvalidXML, t.Name.Local)
				}
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				closed = true
			}
		case xml.CharData:
			if depth == 0 && len(strings.TrimSpace(string(t))) > 0 {
				return fmt.Errorf("%w: text outside root element", domain.ErrInvalidXML)
			}
		}
	}
	if roots != 1 {
		return fmt.Errorf("%w: expected exactly one root element, found %d", domain.ErrInvalidXML, roots)
	}
	if depth != 0 {
		return fmt.Errorf("%w: unclosed elements", domain.ErrInvalidXML)
	}
	return nil
}
