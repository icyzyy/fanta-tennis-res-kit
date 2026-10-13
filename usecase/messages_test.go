package usecase

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

func TestClassifySentinels(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{domain.ErrNotSetDoc, CodeNotSet},
		{domain.ErrBadPad, CodeBadPad},
		{domain.ErrBadCipherSize, CodeCipherSize},
		{domain.ErrReplaceFailed, CodeReplaceFailed},
		{domain.ErrVerifyFailed, CodeVerifyFailed},
		{domain.ErrTextureHeader, CodeBadTexture},
		{domain.ErrTextureFormat, CodeUnsupportedTexture},
		{domain.ErrBadArchive, CodeBadArchive},
		{domain.ErrNoSuchEntry, CodeNotFound},
		{domain.ErrNotEditable, CodeNotEditable},
		{domain.ErrInvalidXML, CodeInvalidXML},
		{domain.ErrBadQuery, CodeBadQuery},
		{domain.ErrBadDestination, CodeBadDestination},
		{domain.ErrNotTexture, CodeNotTexture},
		{fs.ErrNotExist, CodeNotFound},
		{fs.ErrPermission, CodeIO},
		{errors.New("mystery"), CodeInternal},
		// wrapped errors classify by their sentinel
		{fmt.Errorf("read_entry a/b: decrypt: %w", domain.ErrBadPad), CodeBadPad},
	}
	for _, c := range cases {
		got := Classify(c.err)
		if got.Code != c.code {
			t.Errorf("Classify(%v).Code = %q, want %q", c.err, got.Code, c.code)
		}
		if got.Message == "" {
			t.Errorf("Classify(%v).Message is empty", c.err)
		}
	}
}

func TestClassifyNil(t *testing.T) {
	if got := Classify(nil); got != nil {
		t.Errorf("Classify(nil) = %#v, want nil", got)
	}
}

func TestAppErrorError(t *testing.T) {
	e := &AppError{Code: "bad_pad", Message: "decrypt: invalid pad"}
	if e.Error() != "bad_pad: decrypt: invalid pad" {
		t.Errorf("Error() = %q", e.Error())
	}
}
