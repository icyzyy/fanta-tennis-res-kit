package domain

import "fmt"

// SetKey is the AES-128 key the game derives: "TIMOTEI_ZION" + 4 zero bytes.
const SetKey = "TIMOTEI_ZION\x00\x00\x00\x00"

// BlockSize is the AES block size in bytes.
const BlockSize = 16

// ExpectedPad returns the pad length for a plaintext of plainLen bytes:
// (16 - len%16) % 16, so a block-aligned plaintext gets pad 0. This matches
// the game (84/880 corpus files are block-aligned with pad byte 0).
func ExpectedPad(plainLen int) int {
	return (BlockSize - plainLen%BlockSize) % BlockSize
}

// PlainLen validates a .set layout [1 byte pad][ciphertext] and returns the
// plaintext length implied by fileSize and the pad byte.
func PlainLen(fileSize int, pad byte) (int, error) {
	if fileSize < 1 {
		return 0, ErrNotSetDoc
	}
	if pad >= BlockSize {
		return 0, fmt.Errorf("%w: pad byte %d >= %d", ErrBadPad, pad, BlockSize)
	}
	plain := fileSize - 1 - int(pad)
	if plain < 0 {
		return 0, fmt.Errorf("%w: file of %d bytes too small for pad %d", ErrBadPad, fileSize, pad)
	}
	if ExpectedPad(plain) != int(pad) {
		return 0, fmt.Errorf("%w: pad %d inconsistent with plaintext length %d", ErrBadPad, pad, plain)
	}
	return plain, nil
}
