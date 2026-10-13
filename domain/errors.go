package domain

import "errors"

// Sentinel errors for the .set container, archives and textures.
// Inner layers wrap these with %w; drivers classify via usecase.Classify.
var (
	ErrNotSetDoc      = errors.New("not a set document")
	ErrBadPad         = errors.New("invalid pad byte")
	ErrBadCipherSize  = errors.New("invalid ciphertext size")
	ErrReplaceFailed  = errors.New("archive replace failed")
	ErrVerifyFailed   = errors.New("post-write verification failed")
	ErrTextureHeader  = errors.New("invalid texture header")
	ErrTextureFormat  = errors.New("unsupported texture format")
	ErrBadArchive     = errors.New("archive unreadable")
	ErrNoSuchEntry    = errors.New("entry not found")
	ErrNotEditable    = errors.New("entry kind not editable")
	ErrInvalidXML     = errors.New("invalid xml")
	ErrBadQuery       = errors.New("bad search query")
	ErrBadDestination = errors.New("bad destination")
	ErrNotTexture     = errors.New("entry is not a texture")
)
