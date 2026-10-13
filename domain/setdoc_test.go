package domain

import (
	"errors"
	"testing"
)

func TestExpectedPad(t *testing.T) {
	cases := []struct {
		plainLen int
		want     int
	}{
		{0, 0}, {1, 15}, {15, 1}, {16, 0}, {17, 15}, {6115, 13}, {32, 0},
	}
	for _, c := range cases {
		if got := ExpectedPad(c.plainLen); got != c.want {
			t.Errorf("ExpectedPad(%d) = %d, want %d", c.plainLen, got, c.want)
		}
	}
}

func TestPlainLenValid(t *testing.T) {
	// Real fixture: LevelExp_Pet.set is 6129 bytes, pad 13 → 6115 plaintext.
	if n, err := PlainLen(6129, 13); err != nil || n != 6115 {
		t.Errorf("PlainLen(6129,13) = %d, %v; want 6115, nil", n, err)
	}
	// Block-aligned: plain 16, pad 0, file = 1 + 16.
	if n, err := PlainLen(17, 0); err != nil || n != 16 {
		t.Errorf("PlainLen(17,0) = %d, %v; want 16, nil", n, err)
	}
	// Empty plaintext: pad 0, file = 1 byte.
	if n, err := PlainLen(1, 0); err != nil || n != 0 {
		t.Errorf("PlainLen(1,0) = %d, %v; want 0, nil", n, err)
	}
}

func TestPlainLenInvalid(t *testing.T) {
	cases := []struct {
		name     string
		fileSize int
		pad      byte
		want     error
	}{
		{"no header byte", 0, 0, ErrNotSetDoc},
		{"pad >= 16", 40, 16, ErrBadPad},
		{"pad 255", 40, 255, ErrBadPad},
		{"file too small for pad", 5, 10, ErrBadPad},
		{"pad inconsistent with length", 18, 5, ErrBadPad}, // plain=12 needs pad 4, not 5
		{"aligned but pad 16", 33, 16, ErrBadPad},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := PlainLen(c.fileSize, c.pad)
			if !errors.Is(err, c.want) {
				t.Errorf("PlainLen(%d,%d) err = %v, want %v", c.fileSize, c.pad, err, c.want)
			}
		})
	}
}
