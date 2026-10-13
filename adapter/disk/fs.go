package disk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FS implements usecase.FileSystem with the local disk.
type FS struct{}

// New returns the local-filesystem adapter.
func New() *FS { return &FS{} }

// WalkRes returns every *.res file under root (case-insensitive suffix),
// sorted, skipping directories and not following symlinks.
func (f *FS) WalkRes(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".res") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Strings(out)
	return out, nil
}

// Stat returns size and modification time.
func (f *FS) Stat(path string) (int64, time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	return fi.Size(), fi.ModTime(), nil
}

// WriteFile writes data, creating parent directories.
func (f *FS) WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
