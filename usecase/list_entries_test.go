package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func TestListEntries(t *testing.T) {
	repo := &fakeRepo{
		listFn: func(archive string) ([]domain.Entry, error) {
			return []domain.Entry{
				{Name: "a.set", Kind: domain.KindSet, UncompressedSize: 10, CompressedSize: 5, Method: 8, ModTime: time.Unix(100, 0)},
				{Name: "b.tex", Kind: domain.KindTex, UncompressedSize: 20, Method: 0},
			}, nil
		},
	}
	got, err := NewListEntries(repo).Do(context.Background(), "x.res")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(got) != 2 || got[0].Name != "a.set" || got[0].Kind != domain.KindSet {
		t.Fatalf("got = %+v", got)
	}
	if got[0].UncompressedSize != 10 || got[0].Method != 8 || got[0].ModTime != 100 {
		t.Errorf("mapping wrong: %+v", got[0])
	}
	if got[1].Kind != domain.KindTex {
		t.Errorf("kind mapping wrong: %+v", got[1])
	}
}

func TestListEntriesError(t *testing.T) {
	repo := &fakeRepo{
		listFn: func(archive string) ([]domain.Entry, error) {
			return nil, domain.ErrBadArchive
		},
	}
	_, err := NewListEntries(repo).Do(context.Background(), "x.res")
	if !errors.Is(err, domain.ErrBadArchive) {
		t.Errorf("err = %v, want wrapped ErrBadArchive", err)
	}
	if got := err.Error(); len(got) < 12 || got[:12] != "list_entries" {
		t.Errorf("missing context prefix: %v", err)
	}
}
