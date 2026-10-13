package usecase

import (
	"errors"
	"io/fs"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// Error codes carried by AppError across the driver boundary.
const (
	CodeNotSet             = "not_set"
	CodeBadPad             = "bad_pad"
	CodeCipherSize         = "cipher_size"
	CodeReplaceFailed      = "replace_failed"
	CodeVerifyFailed       = "verify_failed"
	CodeBadTexture         = "bad_texture"
	CodeUnsupportedTexture = "unsupported_texture"
	CodeBadArchive         = "bad_archive"
	CodeNotFound           = "not_found"
	CodeNotEditable        = "not_editable"
	CodeInvalidXML         = "invalid_xml"
	CodeBadQuery           = "bad_query"
	CodeBadDestination     = "bad_destination"
	CodeNotTexture         = "not_texture"
	CodeIO                 = "io"
	CodeInternal           = "internal"
)

// AppError is the structured failure that crosses the driver boundary.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }

// Classify maps any wrapped error to an AppError. Returns nil for nil.
// Drivers (CLI, Wails controllers) call this — inner layers never format
// user-facing text.
func Classify(err error) *AppError {
	if err == nil {
		return nil
	}
	code := CodeInternal
	switch {
	case errors.Is(err, domain.ErrNotSetDoc):
		code = CodeNotSet
	case errors.Is(err, domain.ErrBadPad):
		code = CodeBadPad
	case errors.Is(err, domain.ErrBadCipherSize):
		code = CodeCipherSize
	case errors.Is(err, domain.ErrReplaceFailed):
		code = CodeReplaceFailed
	case errors.Is(err, domain.ErrVerifyFailed):
		code = CodeVerifyFailed
	case errors.Is(err, domain.ErrTextureHeader):
		code = CodeBadTexture
	case errors.Is(err, domain.ErrTextureFormat):
		code = CodeUnsupportedTexture
	case errors.Is(err, domain.ErrBadArchive):
		code = CodeBadArchive
	case errors.Is(err, domain.ErrNoSuchEntry), errors.Is(err, fs.ErrNotExist):
		code = CodeNotFound
	case errors.Is(err, domain.ErrNotEditable):
		code = CodeNotEditable
	case errors.Is(err, domain.ErrInvalidXML):
		code = CodeInvalidXML
	case errors.Is(err, domain.ErrBadQuery):
		code = CodeBadQuery
	case errors.Is(err, domain.ErrBadDestination):
		code = CodeBadDestination
	case errors.Is(err, domain.ErrNotTexture):
		code = CodeNotTexture
	case errors.Is(err, fs.ErrPermission):
		code = CodeIO
	}
	return &AppError{Code: code, Message: err.Error()}
}

// --- Boundary messages ---

// Workspace is an opened workspace root.
type Workspace struct {
	Root     string        `json:"root"`
	Archives []ArchiveInfo `json:"archives"`
}

// ArchiveInfo describes one discovered .res file.
type ArchiveInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // unix seconds
}

// EntryInfo describes one entry inside an archive.
type EntryInfo struct {
	Name             string           `json:"name"`
	Kind             domain.EntryKind `json:"kind"`
	CompressedSize   int64            `json:"compressedSize"`
	UncompressedSize int64            `json:"uncompressedSize"`
	Method           uint16           `json:"method"`
	ModTime          int64            `json:"modTime"`
}

// ContentKind discriminates Content payloads.
type ContentKind int

const (
	ContentXML ContentKind = iota
	ContentPNG
	ContentText
	ContentBinary
)

func (k ContentKind) String() string {
	switch k {
	case ContentXML:
		return "xml"
	case ContentPNG:
		return "png"
	case ContentText:
		return "text"
	default:
		return "binary"
	}
}

// Content is an entry's decoded payload.
type Content struct {
	Kind    ContentKind     `json:"kind"`
	Text    string          `json:"text,omitempty"`    // xml/text
	Data    []byte          `json:"data,omitempty"`    // png bytes / raw binary
	Texture *domain.Texture `json:"texture,omitempty"` // png only
}

// SaveRequest is an edit-and-repack request.
type SaveRequest struct {
	Archive string `json:"archive"`
	Entry   string `json:"entry"`
	Text    string `json:"text"`
}

// SaveResult reports a successful save. Revision is CRC32 of the stored
// plaintext (informational revision token for the frontend).
type SaveResult struct {
	OK       bool   `json:"ok"`
	Revision uint32 `json:"revision"`
}

// SearchRequest scopes a global search. Archive != "" narrows to one file.
type SearchRequest struct {
	Root          string `json:"root"`
	Archive       string `json:"archive,omitempty"`
	Query         string `json:"query"`
	CaseSensitive bool   `json:"caseSensitive"`
}

// SearchHit is one matching line, or a warning (Line/Text empty, Warning set).
type SearchHit struct {
	Archive string `json:"archive"`
	Entry   string `json:"entry,omitempty"`
	Line    int    `json:"line,omitempty"`
	Text    string `json:"text,omitempty"`
	Warning string `json:"warning,omitempty"`
}

// ExtractRequest exports entries; empty Entries means the whole archive.
type ExtractRequest struct {
	Archive   string   `json:"archive"`
	Entries   []string `json:"entries,omitempty"`
	Dest      string   `json:"dest"`
	Decrypted bool     `json:"decrypted"`
}

// ExtractResult lists files written.
type ExtractResult struct {
	Files []string `json:"files"`
}
