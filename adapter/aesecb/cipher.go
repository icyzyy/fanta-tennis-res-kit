package aesecb

import (
	"crypto/aes"
	"fmt"

	"github.com/icyzyy/fanta-tennis-res-kit/domain"
)

// Cipher implements the .set container: AES-128-ECB with the game's key and
// zero padding. Layout: [1 byte pad length][ECB(zero-padded plaintext)].
type Cipher struct{}

// New returns the cipher used by the game.
func New() *Cipher { return &Cipher{} }

// Encrypt packs plain into the .set layout.
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	pad := domain.ExpectedPad(len(plain))
	body := make([]byte, len(plain)+pad) // zero padding
	copy(body, plain)

	out := make([]byte, 1+len(body))
	out[0] = byte(pad)

	block, err := aes.NewCipher([]byte(domain.SetKey))
	if err != nil {
		return nil, fmt.Errorf("aesecb: %w", err)
	}
	for i := 0; i < len(body); i += aes.BlockSize {
		block.Encrypt(out[1+i:1+i+aes.BlockSize], body[i:i+aes.BlockSize])
	}
	return out, nil
}

// Decrypt unpacks a .set file to its plaintext.
func (c *Cipher) Decrypt(file []byte) ([]byte, error) {
	if len(file) < 1 {
		return nil, domain.ErrNotSetDoc
	}
	pad := file[0]
	if pad >= domain.BlockSize {
		return nil, fmt.Errorf("%w: pad byte %d >= %d", domain.ErrBadPad, pad, domain.BlockSize)
	}
	body := file[1:]
	if len(body)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: ciphertext of %d bytes not a multiple of %d",
			domain.ErrBadCipherSize, len(body), aes.BlockSize)
	}
	plainLen, err := domain.PlainLen(len(file), pad)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(domain.SetKey))
	if err != nil {
		return nil, fmt.Errorf("aesecb: %w", err)
	}
	out := make([]byte, len(body))
	for i := 0; i < len(body); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], body[i:i+aes.BlockSize])
	}
	return out[:plainLen], nil
}
