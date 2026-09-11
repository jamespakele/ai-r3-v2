package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upRemoveIntakeStatus removes the intake.status select field entirely.
// Its only remaining values after migration 017 are ["unassigned","completed"],
// and the field no longer serves a purpose. Stored status values are dropped
// with the field — there is no data rewrite or preservation step.
func upRemoveIntakeStatus(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Idempotency guard: if the status field is already absent, no-op.
	if intakeCol.Fields.GetByName("status") == nil {
		return nil
	}

	intakeCol.Fields.RemoveByName("status")
	return app.Save(intakeCol)
}

// downRemoveIntakeStatus re-adds intake.status as an optional single-select
// field with values exactly ["unassigned","completed"] (the post-017 shape so
// the two migrations compose in either direction). Dropped stored values are
// not restored.
func downRemoveIntakeStatus(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Idempotency guard: if the status field is already present, no-op.
	if intakeCol.Fields.GetByName("status") != nil {
		return nil
	}

	intakeCol.Fields.Add(&core.SelectField{
		Name:      "status",
		Required:  false,
		MaxSelect: 1,
		Values:    []string{"unassigned", "completed"},
	})
	return app.Save(intakeCol)
}
