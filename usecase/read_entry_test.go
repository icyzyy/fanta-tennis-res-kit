package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func readEntryWith(repo *fakeRepo, cipher *stubCipher, tex *stubTex) *ReadEntry {
	return NewReadEntry(repo, cipher, tex)
}

func TestReadEntrySet(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) {
		return []byte("enc:<a/>"), nil
	}}
	got, err := readEntryWith(repo, &stubCipher{}, &stubTex{}).Do(context.Background(), "g.res", "x.set")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Kind != ContentXML || got.Text != "<a/>" {
		t.Errorf("got = %+v, want xml <a/>", got)
	}
}

func TestReadEntrySetDecryptError(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte("bad"), nil }}
	cipher := &stubCipher{dec: func(b []byte) ([]byte, error) {
		return nil, domain.ErrBadPad
	}}
	_, err := readEntryWith(repo, cipher, &stubTex{}).Do(context.Background(), "g.res", "x.set")
	if !errors.Is(err, domain.ErrBadPad) {
		t.Errorf("err = %v, want wrapped ErrBadPad", err)
	}
	if got := err.Error(); !strings.HasPrefix(got, "read_entry g.res/x.set:") {
		t.Errorf("missing context: %v", err)
	}
}

func TestReadEntryTex(t *testing.T) {
	repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return []byte{0xbb, 0xbb}, nil }}
	tex := &stubTex{meta: &domain.Texture{Width: 4, Height: 4, Format: domain.FmtDXT1}, png: []byte("PNGDATA")}
	got, err := readEntryWith(repo, &stubCipher{}, tex).Do(context.Background(), "g.res", "x.tex")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Kind != ContentPNG || string(got.Data) != "PNGDATA" {
		t.Errorf("got = %+v", got)
	}
	if got.Texture == nil || got.Texture.Width != 4 {
		t.Errorf("texture meta missing: %+v", got.Texture)
	}
}

func TestReadEntryTextAndBinary(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want ContentKind
	}{
		{"plain text", []byte("hello world"), ContentText},
		{"utf8 xml", []byte("<a/>"), ContentText},
		{"empty", nil, ContentText},
		{"nul byte", []byte{'a', 0, 'b'}, ContentBinary},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}, ContentBinary},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{readFn: func(a, e string) ([]byte, error) { return c.in, nil }}
			got, err := readEntryWith(repo, &stubCipher{}, &stubTex{}).Do(context.Background(), "g.res", "x.bin")
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if got.Kind != c.want {
				t.Errorf("Kind = %v, want %v", got.Kind, c.want)
			}
			if c.want == ContentBinary && string(got.Data) != string(c.in) {
				t.Errorf("binary payload lost: %q", got.Data)
			}
		})
	}
}

func TestReadEntryRepoError(t *testing.T) {
	repo := &fakeRepo{} // defaults to ErrNoSuchEntry
	_, err := readEntryWith(repo, &stubCipher{}, &stubTex{}).Do(context.Background(), "g.res", "x.set")
	if !errors.Is(err, domain.ErrNoSuchEntry) {
		t.Errorf("err = %v, want wrapped ErrNoSuchEntry", err)
	}
}

func TestReadEntryCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := readEntryWith(&fakeRepo{}, &stubCipher{}, &stubTex{}).Do(ctx, "g.res", "x.set")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
