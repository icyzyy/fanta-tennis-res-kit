package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// Extract exports entries to a user-chosen destination directory.
type Extract struct {
	repo   ArchiveRepo
	cipher SetCipher
	fs     FileSystem
}

// NewExtract wires the interactor.
func NewExtract(repo ArchiveRepo, cipher SetCipher, fs FileSystem) *Extract {
	return &Extract{repo: repo, cipher: cipher, fs: fs}
}

// Do writes the selected entries (all, when req.Entries is empty) under
// req.Dest. Decrypted mode decrypts .set entries; everything else is copied
// raw. Fails fast: on error, files written so far remain on disk.
func (x *Extract) Do(ctx context.Context, req ExtractRequest, progress ProgressPort) (*ExtractResult, error) {
	if progress == nil {
		progress = ProgressFunc(func(ProgressEvent) {})
	}
	if req.Dest == "" {
		return nil, fmt.Errorf("extract %s: %w", req.Archive, domain.ErrBadDestination)
	}
	ents, err := x.repo.List(req.Archive)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", req.Archive, err)
	}

	selected := ents
	if len(req.Entries) > 0 {
		want := map[string]bool{}
		for _, n := range req.Entries {
			want[n] = true
		}
		selected = selected[:0:0] // fresh slice, don't alias
		found := map[string]bool{}
		for _, e := range ents {
			if want[e.Name] {
				selected = append(selected, e)
				found[e.Name] = true
			}
		}
		for _, n := range req.Entries {
			if !found[n] {
				return nil, fmt.Errorf("extract %s: %w: %s", req.Archive, domain.ErrNoSuchEntry, n)
			}
		}
	}

	res := &ExtractResult{Files: make([]string, 0, len(selected))}
	for i, e := range selected {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		data, err := x.repo.Read(req.Archive, e.Name)
		if err != nil {
			return nil, fmt.Errorf("extract %s/%s: %w", req.Archive, e.Name, err)
		}
		if req.Decrypted && e.Kind == domain.KindSet {
			plain, err := x.cipher.Decrypt(data)
			if err != nil {
				return nil, fmt.Errorf("extract %s/%s: decrypt: %w", req.Archive, e.Name, err)
			}
			data = plain
		}
		dest := filepath.Join(req.Dest, e.Name)
		if err := x.fs.WriteFile(dest, data); err != nil {
			return nil, fmt.Errorf("extract %s/%s: %w", req.Archive, e.Name, err)
		}
		res.Files = append(res.Files, dest)
		progress.Report(ProgressEvent{
			Op: "extract", Done: i + 1, Total: len(selected), Path: dest,
		})
	}
	return res, nil
}
