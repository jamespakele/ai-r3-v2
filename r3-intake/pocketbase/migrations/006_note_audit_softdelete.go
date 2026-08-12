package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upNoteAuditSoftDelete adds a soft-delete flag to the notes collection and
// creates the note_changes audit collection.
func upNoteAuditSoftDelete(app core.App) error {
	// Add deleted bool to notes.
	notesCol, err := app.FindCollectionByNameOrId("notes")
	if err != nil {
		return err
	}
	if notesCol.Fields.GetByName("deleted") == nil {
		notesCol.Fields.Add(&core.BoolField{Name: "deleted", Required: false})
		if err := app.Save(notesCol); err != nil {
			return err
		}
	}

	// Create note_changes collection if it doesn't exist.
	if _, err := app.FindCollectionByNameOrId("note_changes"); err == nil {
		return nil
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	col := core.NewBaseCollection("note_changes")
	col.Fields.Add(&core.RelationField{
		Name:          "note_id",
		CollectionId:  notesCol.Id,
		MaxSelect:     1,
		Required:      true,
		CascadeDelete: false,
	})
	col.Fields.Add(&core.RelationField{
		Name:          "user_id",
		CollectionId:  usersCol.Id,
		MaxSelect:     1,
		Required:      true,
		CascadeDelete: false,
	})
	col.Fields.Add(&core.TextField{
		Name:     "action",
		Required: true,
		Max:      20,
	})
	col.Fields.Add(&core.TextField{
		Name:     "change_from",
		Required: false,
		Max:      20000,
	})
	col.Fields.Add(&core.TextField{
		Name:     "change_to",
		Required: false,
		Max:      20000,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
		OnUpdate: false,
	})
	return app.Save(col)
}

// downNoteAuditSoftDelete removes the soft-delete flag and the audit collection.
func downNoteAuditSoftDelete(app core.App) error {
	notesCol, err := app.FindCollectionByNameOrId("notes")
	if err != nil {
		return err
	}
	if notesCol.Fields.GetByName("deleted") != nil {
		notesCol.Fields.RemoveByName("deleted")
		if err := app.Save(notesCol); err != nil {
			return err
		}
	}

	col, err := app.FindCollectionByNameOrId("note_changes")
	if err != nil {
		return nil
	}
	return app.Delete(col)
}
