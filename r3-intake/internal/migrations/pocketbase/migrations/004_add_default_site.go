package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func upAddDefaultSite(app core.App) error {
	col, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		return err
	}
	if col.Fields.GetByName("is_default") != nil {
		return nil
	}
	col.Fields.Add(&core.BoolField{Name: "is_default", Required: false})
	return app.Save(col)
}

func downAddDefaultSite(app core.App) error {
	col, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		return err
	}
	if col.Fields.GetByName("is_default") == nil {
		return nil
	}
	col.Fields.RemoveByName("is_default")
	return app.Save(col)
}
