package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// searchWorkers bounds concurrent archive decryption.
const searchWorkers = 4

// SearchAll decrypts every .set in scope and streams matching lines.
type SearchAll struct {
	repo   ArchiveRepo
	cipher SetCipher
	fs     FileSystem

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	size    int64
	modTime int64
	docs    []cacheDoc
}

type cacheDoc struct {
	entry string
	text  string
}

// NewSearchAll wires the interactor.
func NewSearchAll(repo ArchiveRepo, cipher SetCipher, fs FileSystem) *SearchAll {
	return &SearchAll{repo: repo, cipher: cipher, fs: fs, cache: map[string]cacheEntry{}}
}

// Do runs the search. emit receives hits as they are found (serialized —
// callers need no locking). progress may be nil. Returns ctx error on
// cancellation; corrupt archives/entries become Warning hits, never errors.
func (x *SearchAll) Do(ctx context.Context, req SearchRequest, emit func(SearchHit), progress ProgressPort) error {
	if req.Query == "" {
		return fmt.Errorf("search_all: %w", domain.ErrBadQuery)
	}
	if progress == nil {
		progress = ProgressFunc(func(ProgressEvent) {})
	}

	var archives []string
	if req.Archive != "" {
		archives = []string{req.Archive}
	} else {
		paths, err := x.fs.WalkRes(req.Root)
		if err != nil {
			return fmt.Errorf("search_all: %w", err)
		}
		archives = paths
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("search_all: %w", err)
	}

	var emitMu sync.Mutex
	safeEmit := func(h SearchHit) {
		emitMu.Lock()
		emit(h)
		emitMu.Unlock()
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < searchWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for arch := range jobs {
				if ctx.Err() != nil {
					continue // drain without working; sender stops on cancel
				}
				x.searchArchive(ctx, arch, req, safeEmit)
			}
		}()
	}

	total := len(archives)
send:
	for _, arch := range archives {
		select {
		case <-ctx.Done():
			break send
		case jobs <- arch:
		}
	}
	close(jobs)
	wg.Wait()

	// Final 100% event (workers report as they go).
	progress.Report(ProgressEvent{Op: "search", Done: total, Total: total, Path: ""})

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("search_all: %w", err)
	}
	return nil
}

// searchArchive decrypts (or reuses) one archive and streams its hits.
func (x *SearchAll) searchArchive(ctx context.Context, archive string, req SearchRequest, emit func(SearchHit)) {
	docs, skipped, err := x.docsFor(archive)
	if err != nil {
		emit(SearchHit{Archive: archive, Warning: err.Error()})
		return
	}
	for _, s := range skipped {
		emit(SearchHit{Archive: archive, Entry: s.entry, Warning: s.reason})
	}
	for _, d := range docs {
		if ctx.Err() != nil {
			return
		}
		for i, line := range strings.Split(d.text, "\n") {
			line = strings.TrimRight(line, "\r")
			if domain.LineMatches(line, req.Query, req.CaseSensitive) {
				emit(SearchHit{Archive: archive, Entry: d.entry, Line: i + 1, Text: strings.TrimSpace(line)})
			}
		}
	}
}

type skippedDoc struct {
	entry  string
	reason string
}

// docsFor returns decrypted .set documents, cached by archive size+mtime.
func (x *SearchAll) docsFor(archive string) ([]cacheDoc, []skippedDoc, error) {
	size, mt, err := x.fs.Stat(archive)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", domain.ErrBadArchive, archive, err)
	}
	x.mu.Lock()
	if c, ok := x.cache[archive]; ok && c.size == size && c.modTime == mt.Unix() {
		x.mu.Unlock()
		return c.docs, nil, nil
	}
	x.mu.Unlock()

	ents, err := x.repo.List(archive)
	if err != nil {
		return nil, nil, err // classified by caller (usually ErrBadArchive)
	}
	var docs []cacheDoc
	var skipped []skippedDoc
	for _, e := range ents {
		if e.Kind != domain.KindSet {
			continue
		}
		b, err := x.repo.Read(archive, e.Name)
		if err != nil {
			skipped = append(skipped, skippedDoc{entry: e.Name, reason: err.Error()})
			continue
		}
		plain, err := x.cipher.Decrypt(b)
		if err != nil {
			skipped = append(skipped, skippedDoc{entry: e.Name, reason: err.Error()})
			continue
		}
		docs = append(docs, cacheDoc{entry: e.Name, text: string(plain)})
	}
	x.mu.Lock()
	x.cache[archive] = cacheEntry{size: size, modTime: mt.Unix(), docs: docs}
	x.mu.Unlock()
	return docs, skipped, nil
}
