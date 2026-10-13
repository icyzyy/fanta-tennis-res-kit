package usecase

import (
	"context"
	"fmt"
)

// ListEntries lazily indexes one archive from its central directory.
type ListEntries struct {
	repo ArchiveRepo
}

// NewListEntries wires the interactor.
func NewListEntries(repo ArchiveRepo) *ListEntries {
	return &ListEntries{repo: repo}
}

// Do returns the archive's entries.
func (x *ListEntries) Do(ctx context.Context, archive string) ([]EntryInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list_entries: %w", err)
	}
	ents, err := x.repo.List(archive)
	if err != nil {
		return nil, fmt.Errorf("list_entries %s: %w", archive, err)
	}
	out := make([]EntryInfo, len(ents))
	for i, e := range ents {
		out[i] = EntryInfo{
			Name:             e.Name,
			Kind:             e.Kind,
			CompressedSize:   e.CompressedSize,
			UncompressedSize: e.UncompressedSize,
			Method:           e.Method,
			ModTime:          e.ModTime.Unix(),
		}
	}
	return out, nil
}
