package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// upAttendanceUniqueIndexes enforces uniqueness at the DB level for
// attendance (one row per event + intake + date) and active event enrollments
// (one active row per event + intake). Existing duplicates must be resolved
// before this migration runs.
func upAttendanceUniqueIndexes(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return fmt.Errorf("find attendance collection: %w", err)
	}
	if attCol.GetIndex("idx_attendance_event_intake_date") == "" {
		attCol.AddIndex("idx_attendance_event_intake_date", true, "event, intake, date", "")
	}
	if err := app.Save(attCol); err != nil {
		return fmt.Errorf("save attendance indexes: %w", err)
	}

	enrollCol, err := app.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		return fmt.Errorf("find event_enrollment collection: %w", err)
	}
	if enrollCol.GetIndex("idx_enrollment_event_intake_active") == "" {
		// Partial index: only active (not soft-deleted) enrollments are unique.
		enrollCol.AddIndex("idx_enrollment_event_intake_active", true, "event, intake", "deleted = false")
	}
	if err := app.Save(enrollCol); err != nil {
		return fmt.Errorf("save event_enrollment indexes: %w", err)
	}

	return nil
}

// downAttendanceUniqueIndexes removes the unique indexes.
func downAttendanceUniqueIndexes(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return fmt.Errorf("find attendance collection: %w", err)
	}
	attCol.RemoveIndex("idx_attendance_event_intake_date")
	if err := app.Save(attCol); err != nil {
		return fmt.Errorf("save attendance indexes: %w", err)
	}

	enrollCol, err := app.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		return fmt.Errorf("find event_enrollment collection: %w", err)
	}
	enrollCol.RemoveIndex("idx_enrollment_event_intake_active")
	if err := app.Save(enrollCol); err != nil {
		return fmt.Errorf("save event_enrollment indexes: %w", err)
	}

	return nil
}
