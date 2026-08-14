package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upUsersDeleted adds a soft-delete flag to the users auth collection.
func upUsersDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("users")
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

// downUsersDeleted removes the soft-delete flag from users.
func downUsersDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("users")
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
