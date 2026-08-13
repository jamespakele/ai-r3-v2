package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upEventEnrollmentDeleted adds a soft-delete flag to the event_enrollment
// collection so unenroll can preserve attendance history (FR12).
func upEventEnrollmentDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		return err
	}
	if col.Fields.GetByName("deleted") == nil {
		col.Fields.Add(&core.BoolField{Name: "deleted", Required: false})
		if err := app.Save(col); err != nil {
			return err
		}
	}
	return nil
}

// downEventEnrollmentDeleted removes the soft-delete flag from
// event_enrollment.
func downEventEnrollmentDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		return err
	}
	if col.Fields.GetByName("deleted") != nil {
		col.Fields.RemoveByName("deleted")
		if err := app.Save(col); err != nil {
			return err
		}
	}
	return nil
}
