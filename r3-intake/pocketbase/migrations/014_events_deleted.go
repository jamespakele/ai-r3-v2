package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upEventsDeleted adds a soft-delete flag to the events collection so events
// can be marked as deleted without removing rows.
func upEventsDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("events")
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

// downEventsDeleted removes the soft-delete flag from events.
func downEventsDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("events")
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
