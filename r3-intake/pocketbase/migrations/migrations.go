package migrations

import (
	"github.com/pocketbase/pocketbase"
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
	migrations.Register(upAttendanceSiteOptional, downAttendanceSiteOptional, "009_attendance_site_optional.go")
	migrations.Register(upAttendanceEventRequired, downAttendanceEventRequired, "010_attendance_event_required.go")
	migrations.Register(upEncryptExistingData, downEncryptExistingData, "011_encrypt_existing_data.go")
	migrations.Register(upExpandSensitiveFieldMax, downExpandSensitiveFieldMax, "012_expand_sensitive_field_max.go")
	migrations.Register(upAttendanceUniqueIndexes, downAttendanceUniqueIndexes, "013_attendance_unique_indexes.go")
	migrations.Register(upUsersDeleted, downUsersDeleted, "014_users_deleted.go")
	migrations.Register(upEventsDeleted, downEventsDeleted, "014_events_deleted.go")
}
