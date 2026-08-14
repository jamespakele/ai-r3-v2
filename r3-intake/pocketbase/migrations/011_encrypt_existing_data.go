package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upEncryptExistingData catches deployments where the original 002 no-op
// migration was already applied. When encryption is now enabled, it first
// expands the sensitive field max sizes, then encrypts every plaintext
// sensitive intake field. Idempotent: already-encrypted values are skipped.
func upEncryptExistingData(app core.App) error {
	if err := upExpandSensitiveFieldMax(app); err != nil {
		return err
	}
	return encryptIntakeRecords(app)
}

// downEncryptExistingData decrypts the same fields back to plaintext on rollback.
func downEncryptExistingData(app core.App) error {
	return decryptIntakeRecords(app)
}
