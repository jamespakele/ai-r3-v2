package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upAttendanceSiteOptional makes the attendance.site relation field optional so
// attendance can be tracked for participants with no assigned intake site.
func upAttendanceSiteOptional(app core.App) error {
	col, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}
	f := col.Fields.GetByName("site")
	if rf, ok := f.(*core.RelationField); ok && rf.Required {
		rf.Required = false
		if err := app.Save(col); err != nil {
			return err
		}
	}
	return nil
}

// downAttendanceSiteOptional restores the required constraint on
// attendance.site. Best-effort: it may fail if empty-site records exist.
func downAttendanceSiteOptional(app core.App) error {
	col, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}
	f := col.Fields.GetByName("site")
	if rf, ok := f.(*core.RelationField); ok && !rf.Required {
		rf.Required = true
		if err := app.Save(col); err != nil {
			return err
		}
	}
	return nil
}
