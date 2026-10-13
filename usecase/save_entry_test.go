package usecase

import (
	"context"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func TestSaveEntrySuccess(t *testing.T) {
	repo := &fakeRepo{} // Read serves what Replace stored (round-trip verify)
	sv := NewSaveEntry(repo, &stubCipher{})
	got, err := sv.Do(context.Background(), SaveRequest{
		Archive: "g.res", Entry: "x.set", Text: "<a><b/></a>",
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !got.OK || got.Revision == 0 {
		t.Errorf("result = %+v", got)
	}
	// Replace must have been called with the ENCRYPTED text.
	log := repo.replacedLog()
	if len(log) != 1 || log[0] != "g.res/x.set" {
		t.Fatalf("replace log = %v", log)
	}
}

func TestSaveEntryRevisionIsCRC32(t *testing.T) {
	repo := &fakeRepo{}
	got, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
		SaveRequest{Archive: "a", Entry: "e.set", Text: "<x/>"})
	if err != nil {
		t.Fatal(err)
	}
	want := crc32Of("<x/>")
	if got.Revision != want {
		t.Errorf("Revision = %d, want crc32 %d", got.Revision, want)
	}
}

func TestSaveEntryRejectsNonSet(t *testing.T) {
	repo := &fakeRepo{}
	_, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
		SaveRequest{Archive: "a", Entry: "img.tex", Text: "<x/>"})
	if !errors.Is(err, domain.ErrNotEditable) {
		t.Errorf("err = %v, want ErrNotEditable", err)
	}
	if len(repo.replacedLog()) != 0 {
		t.Error("Replace must not be called for non-.set entries")
	}
}

func TestSaveEntryInvalidXML(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"whitespace only", "   \n "},
		{"unclosed root", "<a>"},
		{"no root", "just text"},
		{"trailing junk", "<a></a>junk"},
		{"two roots", "<a/><b/>"},
		{"mismatched", "<a><b></a>"},
		{"invalid utf8", "<a>\xff bad</a>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
				SaveRequest{Archive: "a", Entry: "e.set", Text: c.text})
			if !errors.Is(err, domain.ErrInvalidXML) {
				t.Errorf("err = %v, want ErrInvalidXML", err)
			}
			if len(repo.replacedLog()) != 0 {
				t.Error("Replace must not be called for invalid XML")
			}
		})
	}
}

func TestSaveEntryValidEdgeXML(t *testing.T) {
	valid := []string{
		"<a/>",
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ExpList><Exp Value=\"1\"/></ExpList>",
		"<!-- comment --><root>text</root>",
		"<!DOCTYPE a><a/>",
	}
	for _, text := range valid {
		repo := &fakeRepo{}
		if _, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
			SaveRequest{Archive: "a", Entry: "e.set", Text: text}); err != nil {
			t.Errorf("valid xml %q rejected: %v", text, err)
		}
	}
}

func TestSaveEntryVerifyFailsWhenReadbackDiffers(t *testing.T) {
	// readFn takes precedence over the fake's store, so the post-save
	// re-read returns scripted bytes that decrypt to something different.
	repo := &fakeRepo{
		readFn: func(a, e string) ([]byte, error) {
			return []byte("enc:DIFFERENT"), nil
		},
	}
	_, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
		SaveRequest{Archive: "a", Entry: "e.set", Text: "<x/>"})
	if !errors.Is(err, domain.ErrVerifyFailed) {
		t.Errorf("err = %v, want wrapped ErrVerifyFailed", err)
	}
}

func TestSaveEntryReplaceErrorWraps(t *testing.T) {
	repo := &fakeRepo{
		replaceFn: func(a, e string, d []byte) error { return domain.ErrReplaceFailed },
	}
	_, err := NewSaveEntry(repo, &stubCipher{}).Do(context.Background(),
		SaveRequest{Archive: "a", Entry: "e.set", Text: "<x/>"})
	if !errors.Is(err, domain.ErrReplaceFailed) {
		t.Errorf("err = %v, want wrapped ErrReplaceFailed", err)
	}
}

func TestSaveEntryCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewSaveEntry(&fakeRepo{}, &stubCipher{}).Do(ctx,
		SaveRequest{Archive: "a", Entry: "e.set", Text: "<x/>"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func crc32Of(s string) uint32 { return crc32.ChecksumIEEE([]byte(s)) }
