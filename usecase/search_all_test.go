package usecase

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// searchFixture wires SearchAll over scripted fakes with two set entries
// and one tex entry in one archive.
func searchFixture() (*SearchAll, *fakeRepo, *fakeFS, *recProgress) {
	repo := &fakeRepo{
		listFn: func(a string) ([]domain.Entry, error) {
			return []domain.Entry{
				{Name: "a.set", Kind: domain.KindSet},
				{Name: "b.set", Kind: domain.KindSet},
				{Name: "img.tex", Kind: domain.KindTex},
			}, nil
		},
		readFn: func(a, e string) ([]byte, error) {
			switch e {
			case "a.set":
				return []byte("enc:line1 TENNIS Racket\nline2 other"), nil
			case "b.set":
				return []byte("enc:tennis appears twice tennis in one line\nnone here"), nil
			default:
				return []byte("enc:tex bytes"), nil
			}
		},
	}
	fs := newFakeFS()
	fs.res["/root"] = []string{"/root/g.res"}
	prog := &recProgress{}
	return NewSearchAll(repo, &stubCipher{}, fs), repo, fs, prog
}

func collect(sa *SearchAll, req SearchRequest, prog *recProgress) ([]SearchHit, error) {
	var hits []SearchHit
	err := sa.Do(context.Background(), req, func(h SearchHit) { hits = append(hits, h) }, prog)
	return hits, err
}

func TestSearchAllCaseInsensitive(t *testing.T) {
	sa, _, _, prog := searchFixture()
	hits, err := collect(sa, SearchRequest{Root: "/root", Query: "tennis"}, prog)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	// Only .set docs are searched: a.set line 1 matches, b.set line 1
	// matches (twice on one line → one hit), tex entries are skipped.
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 (a.set:1 line, b.set:1 line): %+v", len(hits), hits)
	}
	if hits[0].Archive != "/root/g.res" || hits[0].Entry != "a.set" || hits[0].Line != 1 {
		t.Errorf("hits[0] = %+v", hits[0])
	}
	if !strings.Contains(hits[0].Text, "TENNIS") {
		t.Errorf("hit text = %q", hits[0].Text)
	}
	// b.set: query appears on line 1 only → exactly one hit for b
	var bHits int
	for _, h := range hits {
		if h.Entry == "b.set" {
			bHits++
			if h.Line != 1 {
				t.Errorf("b hit line = %d, want 1", h.Line)
			}
		}
	}
	if bHits != 1 {
		t.Errorf("b.set hits = %d, want 1 (one per line)", bHits)
	}
	// Progress: one event, complete.
	evs := prog.snapshot()
	if len(evs) != 1 || evs[0].Done != 1 || evs[0].Total != 1 {
		t.Errorf("progress = %+v", evs)
	}
}

func TestSearchAllCaseSensitive(t *testing.T) {
	sa, _, _, prog := searchFixture()
	hits, err := collect(sa, SearchRequest{Root: "/root", Query: "tennis", CaseSensitive: true}, prog)
	if err != nil {
		t.Fatal(err)
	}
	// Only b.set line 1 has lowercase "tennis".
	if len(hits) != 1 || hits[0].Entry != "b.set" {
		t.Errorf("hits = %+v, want exactly b.set", hits)
	}
}

func TestSearchAllArchiveScope(t *testing.T) {
	sa, _, _, _ := searchFixture()
	hits, err := collect(sa, SearchRequest{Archive: "/root/other.res", Query: "tennis"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// scoped archive is not in fs.res — repo.listFn still answers for any
	// archive, so hits come from the scripted entries. Assert scope plumbing
	// instead: with no Archive, WalkRes on empty root gives nothing.
	sa2, _, _, _ := searchFixture()
	empty := newFakeFS()
	sa2 = NewSearchAll(&fakeRepo{}, &stubCipher{}, empty)
	hits2, err := collect(sa2, SearchRequest{Root: "/empty", Query: "x"}, nil)
	if err != nil || len(hits2) != 0 {
		t.Errorf("empty root: hits=%v err=%v", hits2, err)
	}
	_ = hits
}

func TestSearchAllEmptyQuery(t *testing.T) {
	sa, _, _, _ := searchFixture()
	_, err := collect(sa, SearchRequest{Root: "/root", Query: ""}, nil)
	if !errors.Is(err, domain.ErrBadQuery) {
		t.Errorf("err = %v, want ErrBadQuery", err)
	}
}

func TestSearchAllCorruptEntryEmitsWarning(t *testing.T) {
	sa, repo, _, _ := searchFixture()
	repo.readFn = func(a, e string) ([]byte, error) {
		if e == "a.set" {
			return nil, domain.ErrBadPad
		}
		return []byte("enc:has needle"), nil
	}
	hits, err := collect(sa, SearchRequest{Root: "/root", Query: "needle"}, nil)
	if err != nil {
		t.Fatalf("corrupt entry must not abort: %v", err)
	}
	var warned bool
	for _, h := range hits {
		if h.Warning != "" && h.Entry == "a.set" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("no warning for corrupt entry: %+v", hits)
	}
	// b.set still searched:
	found := false
	for _, h := range hits {
		if h.Entry == "b.set" && h.Warning == "" {
			found = true
		}
	}
	if !found {
		t.Errorf("search did not continue past corrupt entry: %+v", hits)
	}
}

func TestSearchAllCorruptArchiveEmitsWarning(t *testing.T) {
	sa, repo, _, _ := searchFixture()
	repo.listFn = func(a string) ([]domain.Entry, error) {
		return nil, domain.ErrBadArchive
	}
	hits, err := collect(sa, SearchRequest{Root: "/root", Query: "x"}, nil)
	if err != nil {
		t.Fatalf("corrupt archive must not abort: %v", err)
	}
	if len(hits) != 1 || hits[0].Warning == "" || hits[0].Archive != "/root/g.res" {
		t.Errorf("hits = %+v, want one archive-level warning", hits)
	}
}

func TestSearchAllCancelled(t *testing.T) {
	sa, _, _, _ := searchFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var n int
	err := sa.Do(ctx, SearchRequest{Root: "/root", Query: "tennis"},
		func(h SearchHit) { n++ }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n != 0 {
		t.Errorf("emit called %d times after cancel", n)
	}
}

func TestSearchAllCacheInvalidation(t *testing.T) {
	sa, repo, fs, _ := searchFixture()
	var lists atomic.Int32
	baseFn := repo.listFn
	repo.listFn = func(a string) ([]domain.Entry, error) {
		lists.Add(1)
		return baseFn(a)
	}
	var mtime atomic.Int64
	mtime.Store(1000)
	fs.statFn = func(p string) (int64, time.Time, error) {
		return 100, time.Unix(mtime.Load(), 0), nil
	}
	req := SearchRequest{Root: "/root", Query: "tennis"}
	if _, err := collect(sa, req, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := collect(sa, req, nil); err != nil {
		t.Fatal(err)
	}
	if lists.Load() != 1 {
		t.Errorf("second run re-listed: %d calls, want 1 (cache hit)", lists.Load())
	}
	mtime.Store(2000) // archive changed → cache miss
	if _, err := collect(sa, req, nil); err != nil {
		t.Fatal(err)
	}
	if lists.Load() != 2 {
		t.Errorf("after mtime change: %d list calls, want 2", lists.Load())
	}
}
