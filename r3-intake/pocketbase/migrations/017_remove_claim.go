package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upRemoveClaim removes the "claim" concept from the intake collection:
//  1. rewrites every record with status='claimed' to status='unassigned',
//  2. drops "claimed" from the status select field's allowed values,
//  3. removes the assigned_to relation field.
//
// The data rewrite MUST happen before "claimed" is dropped from the enum:
// records still holding status='claimed' would become invalid once the
// option no longer exists.
func upRemoveClaim(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Step 1 — data rewrite: every claimed record becomes unassigned.
	// Idempotent by construction: once the option is dropped, no record can
	// match status='claimed', so a second run finds zero records.
	recs, err := app.FindRecordsByFilter(intakeCol.Id, "status='claimed'", "", 100000, 0)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		rec.Set("status", "unassigned")
		if err := app.Save(rec); err != nil {
			return err
		}
	}

	// Step 2 — drop "claimed" from the status select values. Guarded: if the
	// field is missing, not a select, or the value is already absent, skip
	// (do NOT return — step 3 must still run on a re-run).
	field := intakeCol.Fields.GetByName("status")
	if field != nil {
		sf, ok := field.(*core.SelectField)
		if ok {
			hasClaimed := false
			for _, v := range sf.Values {
				if v == "claimed" {
					hasClaimed = true
				}
			}
			if hasClaimed {
				filtered := sf.Values[:0]
				for _, v := range sf.Values {
					if v != "claimed" {
						filtered = append(filtered, v)
					}
				}
				sf.Values = filtered
				if err := app.Save(intakeCol); err != nil {
					return err
				}
			}
		}
	}

	// Step 3 — remove the assigned_to relation field. Guarded: if the field
	// is already absent, no-op.
	if intakeCol.Fields.GetByName("assigned_to") == nil {
		return nil
	}
	intakeCol.Fields.RemoveByName("assigned_to")
	return app.Save(intakeCol)
}

// downRemoveClaim reverses upRemoveClaim:
//  1. re-adds "claimed" to the intake status select values,
//  2. re-adds intake.assigned_to as an optional single-select relation to
//     users.

// Both steps are idempotent; each guards on current state so a second run is
// a no-op.
func downRemoveClaim(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Step 1 — re-add "claimed" to the status select values if absent
	// (mirrors 003's up guard). Skip-continue: if the status field is missing
	// or not a select, proceed to step 2 anyway.
	field := intakeCol.Fields.GetByName("status")
	if field != nil {
		sf, ok := field.(*core.SelectField)
		if ok {
			hasClaimed := false
			for _, v := range sf.Values {
				if v == "claimed" {
					hasClaimed = true
				}
			}
			if !hasClaimed {
				sf.Values = append(sf.Values, "claimed")
				if err := app.Save(intakeCol); err != nil {
					return err
				}
			}
		}
	}

	// Step 2 — re-add the assigned_to relation field. Guarded: if already
	// present, no-op.
	if intakeCol.Fields.GetByName("assigned_to") != nil {
		return nil
	}
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	intakeCol.Fields.Add(&core.RelationField{
		Name:          "assigned_to",
		CollectionId:  usersCol.Id,
		Required:      false,
		MaxSelect:     1,
		CascadeDelete: false,
	})
	return app.Save(intakeCol)
}
