package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upAddIntakeRole adds "intake" to the allowed values of the users.role
// select field so the schema matches the application code.
func upAddIntakeRole(app core.App) error {
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	field := col.Fields.GetByName("role")
	if field == nil {
		return nil
	}
	sf, ok := field.(*core.SelectField)
	if !ok {
		return nil
	}
	for _, v := range sf.Values {
		if v == "intake" {
			return nil // already present
		}
	}
	sf.Values = append(sf.Values, "intake")
	return app.Save(col)
}

// downAddIntakeRole removes "intake" from the users.role select values.
func downAddIntakeRole(app core.App) error {
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	field := col.Fields.GetByName("role")
	if field == nil {
		return nil
	}
	sf, ok := field.(*core.SelectField)
	if !ok {
		return nil
	}
	filtered := sf.Values[:0]
	for _, v := range sf.Values {
		if v != "intake" {
			filtered = append(filtered, v)
		}
	}
	sf.Values = filtered
	return app.Save(col)
}
