package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upSitesDeleted adds a soft-delete flag to the sites collection so sites
// can be marked as deleted without removing rows.
func upSitesDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("sites")
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

// downSitesDeleted removes the soft-delete flag from sites.
func downSitesDeleted(app core.App) error {
	col, err := app.FindCollectionByNameOrId("sites")
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
