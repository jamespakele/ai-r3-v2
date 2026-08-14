package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upEncryption re-encrypts existing intake sensitive fields when encryption is
// enabled (R3_ENCRYPTION_ENABLED=1 + R3_ENCRYPTION_KEY). It is a no-op when
// encryption is off.
func upEncryption(app core.App) error {
	return encryptIntakeRecords(app)
}

// downEncryption decrypts existing intake sensitive fields back to plaintext
// when rolling back. It requires R3_ENCRYPTION_KEY.
func downEncryption(app core.App) error {
	return decryptIntakeRecords(app)
}
