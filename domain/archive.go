package domain

import (
	"path/filepath"
	"strings"
	"time"
)

// EntryKind classifies archive entries by extension.
type EntryKind int

const (
	KindOther EntryKind = iota
	KindSet
	KindTex
)

func (k EntryKind) String() string {
	switch k {
	case KindSet:
		return "set"
	case KindTex:
		return "tex"
	default:
		return "other"
	}
}

// KindOf classifies an entry by its file extension (case-insensitive).
func KindOf(name string) EntryKind {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".set":
		return KindSet
	case ".tex":
		return KindTex
	default:
		return KindOther
	}
}

// Entry describes one file inside a .res archive (metadata only).
type Entry struct {
	Name             string
	Kind             EntryKind
	CompressedSize   int64
	UncompressedSize int64
	Method           uint16
	ModTime          time.Time
}
