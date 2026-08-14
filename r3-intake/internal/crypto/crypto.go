// Package crypto defines the at-rest cipher seam for sensitive intake fields.
//
// All repository save/load paths call Cipher.Encrypt/Decrypt on the configured
// SensitiveFields (see internal/config). They do not know whether encryption is
// on — the active Cipher is injected from config. With the default
// EncryptionConfig.Enabled=false the injected cipher is PlainCipher, which
// passes bytes through unchanged: zero runtime cost, and switching encryption
// on later is a config flip plus migration 002/011, not a rewrite.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"r3-intake/internal/config"
)

// Cipher is the at-rest transform applied to sensitive field bytes on save and
// the inverse on load.
type Cipher interface {
	Encrypt(plain []byte) ([]byte, error)
	Decrypt(cipher []byte) ([]byte, error)
}

// PlainCipher is the no-op Cipher used when Encryption.Enabled is false.
type PlainCipher struct{}

// NewPlainCipher returns a pass-through Cipher.
func NewPlainCipher() *PlainCipher { return &PlainCipher{} }

// Encrypt returns plain unchanged.
func (c *PlainCipher) Encrypt(plain []byte) ([]byte, error) { return plain, nil }

// Decrypt returns cipher unchanged.
func (c *PlainCipher) Decrypt(cipher []byte) ([]byte, error) { return cipher, nil }

const (
	// encryptedPrefix is a versioned marker prepended to every AES-GCM value.
	// It makes ciphertext instantly distinguishable from plaintext and keeps
	// the encryption migration idempotent.
	encryptedPrefix = "encv1:"

	// keySizes accepted by AES.
	keySize16 = 16
	keySize24 = 24
	keySize32 = 32
)

// AESCipher encrypts with AES-256-GCM (or AES-128/192-GCM depending on key
// length). Ciphertext is stored as "encv1:" + base64(nonce || sealed).
type AESCipher struct {
	aead cipher.AEAD
}

// NewAESCipher returns a Cipher backed by AES-GCM. Key must be 16, 24, or 32
// bytes.
func NewAESCipher(key []byte) (*AESCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESCipher{aead: aead}, nil
}

// Encrypt returns the AES-GCM ciphertext prefixed with encryptedPrefix.
func (c *AESCipher) Encrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return plain, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := c.aead.Seal(nil, nonce, plain, nil)
	blob := make([]byte, len(nonce)+len(sealed))
	copy(blob, nonce)
	copy(blob[len(nonce):], sealed)
	return []byte(encryptedPrefix + base64.StdEncoding.EncodeToString(blob)), nil
}

// Decrypt removes the prefix, base64-decodes, and opens the AES-GCM seal.
func (c *AESCipher) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return blob, nil
	}
	s := string(blob)
	if !strings.HasPrefix(s, encryptedPrefix) {
		return nil, errors.New("ciphertext missing version prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(s[len(encryptedPrefix):])
	if err != nil {
		return nil, err
	}
	if len(raw) < c.aead.NonceSize() {
		return nil, errors.New("ciphertext shorter than nonce")
	}
	nonce := raw[:c.aead.NonceSize()]
	ct := raw[c.aead.NonceSize():]
	return c.aead.Open(nil, nonce, ct, nil)
}

// IsCiphertext reports whether b already carries the encrypted version prefix.
func IsCiphertext(b []byte) bool {
	return strings.HasPrefix(string(b), encryptedPrefix)
}

// NewCipher returns the Cipher selected by cfg.Encryption.
func NewCipher(cfg config.Config) (Cipher, error) {
	if !cfg.Encryption.Enabled {
		return NewPlainCipher(), nil
	}
	key := cfg.Encryption.Key
	switch len(key) {
	case keySize16, keySize24, keySize32:
		// ok
	default:
		return nil, fmt.Errorf("R3_ENCRYPTION_KEY must be %d, %d, or %d bytes; got %d", keySize16, keySize24, keySize32, len(key))
	}
	return NewAESCipher(key)
}
