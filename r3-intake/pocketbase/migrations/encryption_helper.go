package migrations

import (
	"fmt"
	"os"

	"github.com/pocketbase/pocketbase/core"

	"r3-intake/internal/config"
	"r3-intake/internal/crypto"
)

// aesCipherFromEnv builds an AES-GCM Cipher when the operator has supplied a
// key and enabled encryption. Returns nil, nil when encryption is off.
func aesCipherFromEnv() (crypto.Cipher, error) {
	key := os.Getenv("R3_ENCRYPTION_KEY")
	if key == "" {
		return nil, nil
	}
	env := os.Getenv("R3_ENCRYPTION_ENABLED")
	if env != "1" && env != "true" {
		return nil, nil
	}
	cfg := config.Default()
	cfg.Encryption.Enabled = true
	cfg.Encryption.Key = []byte(key)
	return crypto.NewCipher(cfg)
}

// encryptIntakeRecords re-encrypts every plaintext sensitive field in the
// intake collection. It is idempotent: values already carrying the ciphertext
// prefix are skipped.
func encryptIntakeRecords(app core.App) error {
	c, err := aesCipherFromEnv()
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}
	if c == nil {
		return nil
	}

	col, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return fmt.Errorf("find intake collection: %w", err)
	}

	recs, err := app.FindRecordsByFilter(col.Id, "1=1", "", 100000, 0)
	if err != nil {
		return fmt.Errorf("load intake records: %w", err)
	}

	fields := config.Default().Encryption.SensitiveFields
	for _, rec := range recs {
		changed := false
		for _, f := range fields {
			v := rec.GetString(f)
			if v == "" || crypto.IsCiphertext([]byte(v)) {
				continue
			}
			enc, err := c.Encrypt([]byte(v))
			if err != nil {
				return fmt.Errorf("encrypt %s on %s: %w", f, rec.Id, err)
			}
			rec.Set(f, string(enc))
			changed = true
		}
		if changed {
			if err := app.Save(rec); err != nil {
				return fmt.Errorf("save encrypted record %s: %w", rec.Id, err)
			}
		}
	}
	return nil
}

// decryptIntakeRecords reverses encryptIntakeRecords, restoring plaintext.
func decryptIntakeRecords(app core.App) error {
	key := os.Getenv("R3_ENCRYPTION_KEY")
	if key == "" {
		return nil
	}
	cfg := config.Default()
	cfg.Encryption.Enabled = true
	cfg.Encryption.Key = []byte(key)
	c, err := crypto.NewCipher(cfg)
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}

	col, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return fmt.Errorf("find intake collection: %w", err)
	}

	recs, err := app.FindRecordsByFilter(col.Id, "1=1", "", 100000, 0)
	if err != nil {
		return fmt.Errorf("load intake records: %w", err)
	}

	fields := config.Default().Encryption.SensitiveFields
	for _, rec := range recs {
		changed := false
		for _, f := range fields {
			v := rec.GetString(f)
			if v == "" || !crypto.IsCiphertext([]byte(v)) {
				continue
			}
			dec, err := c.Decrypt([]byte(v))
			if err != nil {
				return fmt.Errorf("decrypt %s on %s: %w", f, rec.Id, err)
			}
			rec.Set(f, string(dec))
			changed = true
		}
		if changed {
			if err := app.Save(rec); err != nil {
				return fmt.Errorf("save decrypted record %s: %w", rec.Id, err)
			}
		}
	}
	return nil
}
