// Package migrations registers the Go-side PocketBase migrations for the
// R3 Intake app. JavaScript migrations (001_init.js) live alongside this file
// and are loaded by the jsvm plugin from the migrations dir; Go migrations are
// registered here and run with the JS ones during PocketBase bootstrap (v0.39
// auto-applies pending migrations on start — no --automigrate flag).
package migrations

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// Register registers all Go migrations with the PocketBase app. The filename
// is passed last per the v0.39 migrations.Register signature.
func Register(app *pocketbase.PocketBase) {
	migrations.Register(upEncryption, downEncryption, "002_encryption.go")
	migrations.Register(upAddIntakeRole, downAddIntakeRole, "003_add_intake_role.go")
	migrations.Register(upAddDefaultSite, downAddDefaultSite, "004_add_default_site.go")
	migrations.Register(upNoteAuditSoftDelete, downNoteAuditSoftDelete, "006_note_audit_softdelete.go")
	migrations.Register(upEventEnrollmentDeleted, downEventEnrollmentDeleted, "008_event_enrollment_deleted.go")
}

// upEncryption is a no-op today. The encryption seam (internal/crypto) is wired
// through every repository save/load path with a PlainCipher, so no data shape
// change is needed to turn encryption on.
//
// TODO: when Encryption.Enabled, re-encrypt existing sensitive rows:
//   - load every intake record
//   - for each field in config.Encryption.SensitiveFields (ssn, dob,
//     participantSigDataUrl, casemanagerSigDataUrl), read the current (plaintext)
//     value, run Cipher.Encrypt, and write it back
//   - this migration must run after the active Cipher is swapped from PlainCipher
//     to the AES-GCM implementation keyed by R3_ENCRYPTION_KEY
func upEncryption(app core.App) error {
	return nil
}

// downEncryption reverses the re-encryption (decrypt back to plaintext) if the
// feature is rolled back. No-op today.
//
// TODO: when rolling back, decrypt every sensitive field in place.
func downEncryption(app core.App) error {
	return nil
}