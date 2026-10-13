package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func extractFixture() (*Extract, *fakeRepo, *fakeFS, *recProgress) {
	repo := &fakeRepo{
		listFn: func(a string) ([]domain.Entry, error) {
			return []domain.Entry{
				{Name: "a.set", Kind: domain.KindSet},
				{Name: "sub/b.tex", Kind: domain.KindTex},
				{Name: "c.txt", Kind: domain.KindOther},
			}, nil
		},
		readFn: func(a, e string) ([]byte, error) {
			switch e {
			case "a.set":
				return []byte("enc:<root/>"), nil
			default:
				return []byte("raw:" + e), nil
			}
		},
	}
	fs := newFakeFS()
	return NewExtract(repo, &stubCipher{}, fs), repo, fs, &recProgress{}
}

func TestExtractWholeArchiveRaw(t *testing.T) {
	x, _, fs, prog := extractFixture()
	dest := filepath.Join(t.TempDir(), "out")
	res, err := x.Do(context.Background(), ExtractRequest{
		Archive: "g.res", Dest: dest,
	}, prog)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(res.Files) != 3 {
		t.Fatalf("files = %v, want 3", res.Files)
	}
	// Raw mode: .set stays encrypted.
	if got := fs.written[filepath.Join(dest, "a.set")]; string(got) != "enc:<root/>" {
		t.Errorf("raw a.set = %q", got)
	}
	if got := fs.written[filepath.Join(dest, "sub/b.tex")]; string(got) != "raw:sub/b.tex" {
		t.Errorf("b.tex = %q", got)
	}
	evs := prog.snapshot()
	if len(evs) != 3 || evs[0].Op != "extract" || evs[2].Done != 3 {
		t.Errorf("progress = %+v", evs)
	}
}

func TestExtractDecrypted(t *testing.T) {
	x, _, fs, _ := extractFixture()
	dest := filepath.Join(t.TempDir(), "out")
	if _, err := x.Do(context.Background(), ExtractRequest{
		Archive: "g.res", Dest: dest, Decrypted: true,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if got := fs.written[filepath.Join(dest, "a.set")]; string(got) != "<root/>" {
		t.Errorf("decrypted a.set = %q, want <root/>", got)
	}
	// Non-.set entries are copied raw even in decrypted mode.
	if got := fs.written[filepath.Join(dest, "c.txt")]; string(got) != "raw:c.txt" {
		t.Errorf("c.txt = %q", got)
	}
}

func TestExtractSubset(t *testing.T) {
	x, _, fs, _ := extractFixture()
	dest := filepath.Join(t.TempDir(), "out")
	res, err := x.Do(context.Background(), ExtractRequest{
		Archive: "g.res", Dest: dest, Entries: []string{"c.txt"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || len(fs.written) != 1 {
		t.Errorf("files=%v written=%d, want exactly c.txt", res.Files, len(fs.written))
	}
}

func TestExtractMissingRequestedEntry(t *testing.T) {
	x, _, _, _ := extractFixture()
	_, err := x.Do(context.Background(), ExtractRequest{
		Archive: "g.res", Dest: t.TempDir(), Entries: []string{"ghost.set"},
	}, nil)
	if !errors.Is(err, domain.ErrNoSuchEntry) {
		t.Errorf("err = %v, want ErrNoSuchEntry", err)
	}
}

func TestExtractBadDestination(t *testing.T) {
	x, _, _, _ := extractFixture()
	_, err := x.Do(context.Background(), ExtractRequest{Archive: "g.res", Dest: ""}, nil)
	if !errors.Is(err, domain.ErrBadDestination) {
		t.Errorf("err = %v, want ErrBadDestination", err)
	}
}

func TestExtractListError(t *testing.T) {
	x, repo, _, _ := extractFixture()
	repo.listFn = func(a string) ([]domain.Entry, error) { return nil, domain.ErrBadArchive }
	_, err := x.Do(context.Background(), ExtractRequest{Archive: "g.res", Dest: t.TempDir()}, nil)
	if !errors.Is(err, domain.ErrBadArchive) {
		t.Errorf("err = %v, want wrapped ErrBadArchive", err)
	}
}

func TestExtractCancelled(t *testing.T) {
	x, _, _, _ := extractFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := x.Do(ctx, ExtractRequest{Archive: "g.res", Dest: t.TempDir()}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
