// Package crypto defines the at-rest cipher seam for sensitive intake fields.
//
// All repository save/load paths call Cipher.Encrypt/Decrypt on the configured
// SensitiveFields (see internal/config). They do not know whether encryption is
// on — the active Cipher is injected from config. With the default
// EncryptionConfig.Enabled=false the injected cipher is PlainCipher, which
// passes bytes through unchanged: zero runtime cost, and switching encryption
// on later is a config flip plus migration 002, not a rewrite.
package crypto

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