package aesecb

import (
	"bytes"
	"crypto/aes"
	"errors"
	"os"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatalf("missing fixture %s: %v", name, err)
	}
	return b
}

func TestDecryptGoldenFixture(t *testing.T) {
	got, err := New().Decrypt(fixture(t, "LevelExp_Pet.set"))
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	want := fixture(t, "LevelExp_Pet.xml")
	if !bytes.Equal(got, want) {
		t.Errorf("decrypt mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

func TestEncryptGoldenFixtureByteIdentical(t *testing.T) {
	// This fixture has zero pad bytes, so re-encryption must reproduce the
	// original file byte-for-byte.
	plain := fixture(t, "LevelExp_Pet.xml")
	got, err := New().Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !bytes.Equal(got, fixture(t, "LevelExp_Pet.set")) {
		t.Errorf("Encrypt did not reproduce the original .set bytes")
	}
}

func TestRoundTripLengths(t *testing.T) {
	for _, n := range []int{0, 1, 15, 16, 17, 32, 6115} {
		plain := bytes.Repeat([]byte("x"), n)
		enc, err := New().Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%d): %v", n, err)
		}
		dec, err := New().Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt after %d: %v", n, err)
		}
		if !bytes.Equal(dec, plain) {
			t.Errorf("round-trip of %d bytes failed (got %d)", n, len(dec))
		}
	}
}

func TestEncryptPadHeader(t *testing.T) {
	cases := []struct {
		plainLen int
		wantPad  byte
		wantSize int
	}{
		{0, 0, 1}, {15, 1, 17}, {16, 0, 17}, {17, 15, 33},
	}
	for _, c := range cases {
		enc, err := New().Encrypt(bytes.Repeat([]byte("a"), c.plainLen))
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if enc[0] != c.wantPad {
			t.Errorf("len %d: pad byte = %d, want %d", c.plainLen, enc[0], c.wantPad)
		}
		if len(enc) != c.wantSize {
			t.Errorf("len %d: size = %d, want %d", c.plainLen, len(enc), c.wantSize)
		}
	}
}

func TestDecryptErrors(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, domain.ErrNotSetDoc},
		{"pad byte exceeds block", []byte{16, 1}, domain.ErrBadPad},
		{"body not multiple of 16", []byte{0, 1, 2, 3}, domain.ErrBadCipherSize},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New().Decrypt(c.in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestDecryptIgnoresPadContent(t *testing.T) {
	// The game ignores pad content (838/880 corpus files carry garbage there):
	// a file whose plaintext pad bytes are not zero still decrypts correctly,
	// because only the pad length matters.
	plain := []byte("<a></a>")
	enc, err := New().Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	pad := enc[0]
	if pad == 0 {
		t.Skip("plaintext is block-aligned; no pad region to garbage")
	}
	// Re-encrypt with garbage pad content: [pad][ECB(plain || 0xAB*pad)].
	body := append(append([]byte{}, plain...), bytes.Repeat([]byte{0xAB}, int(pad))...)
	garbled := []byte{pad}
	block, err := aes.NewCipher([]byte(domain.SetKey))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(body); i += aes.BlockSize {
		blk := make([]byte, aes.BlockSize)
		copy(blk, body[i:i+aes.BlockSize])
		encBlock := make([]byte, aes.BlockSize)
		block.Encrypt(encBlock, blk)
		garbled = append(garbled, encBlock...)
	}
	got, err := New().Decrypt(garbled)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("got %q, want %q", got, plain)
	}
}
