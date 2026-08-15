package migrations

import (
	"fmt"

	mcpmod "r3-intake/internal/mcp"

	"github.com/pocketbase/pocketbase/core"
)

// upIntakeSiteToEvent renames intake.site (a relation to sites) to
// intake.event (a relation to events). The intake's home event replaces the
// intake's home site as the roster-scoping and required-field key; the
// event's own site field remains the authoritative location.
//
// Backfill is best-effort: each intake's old site value is mapped to the
// first active, non-deleted event at that site (ordered by start_date, then
// name). Intakes with no matching active event keep event empty (the field is
// optional) and a warning is logged so the operator can reconcile.
func upIntakeSiteToEvent(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Idempotency guard: if the event field is already present, no-op.
	if intakeCol.Fields.GetByName("event") != nil {
		return nil
	}

	eventsCol, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}

	// Capture each intake's old site value before the field is removed.
	recs, err := app.FindRecordsByFilter(intakeCol.Id, "", "", 100000, 0)
	if err != nil {
		return err
	}
	oldSites := make(map[string]string, len(recs))
	for _, rec := range recs {
		oldSites[rec.Id] = rec.GetString("site")
	}

	// Remove the old field and add the new one as an optional single-select
	// relation to events (matching the old site field's optionality).
	intakeCol.Fields.RemoveByName("site")
	intakeCol.Fields.Add(&core.RelationField{
		Name:          "event",
		CollectionId:  eventsCol.Id,
		Required:      false,
		MaxSelect:     1,
		CascadeDelete: false,
	})
	if err := app.Save(intakeCol); err != nil {
		return err
	}

	// Backfill (best-effort): re-load fresh records (post-schema) and map each
	// intake's old site to an active, non-deleted event at that site.
	recs, err = app.FindRecordsByFilter(intakeCol.Id, "", "", 100000, 0)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		oldSite := oldSites[rec.Id]
		if oldSite == "" {
			continue
		}
		filter := fmt.Sprintf("site='%s' && status='active' && (deleted = false || deleted = null)",
			mcpmod.EscapeFilter(oldSite))
		evs, err := app.FindRecordsByFilter(eventsCol.Id, filter, "start_date,name", 1, 0)
		if err != nil {
			app.Logger().Warn("intake-site-to-event: could not query events for backfill", "record_id", rec.Id, "error", err)
			continue
		}
		if len(evs) == 0 {
			app.Logger().Warn("intake-site-to-event: no active event at site; leaving event empty", "record_id", rec.Id, "site", oldSite)
			continue
		}
		rec.Set("event", evs[0].Id)
		if err := app.Save(rec); err != nil {
			return err
		}
	}

	return nil
}

// downIntakeSiteToEvent restores intake.site as an optional relation to sites
// and backfills each intake's site from its event's site so the rollback is
// lossless. Backfill is best-effort: intakes whose event cannot be resolved
// or whose event has no site keep site empty (it is optional).
func downIntakeSiteToEvent(app core.App) error {
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return err
	}

	// Idempotency guard: if the site field is already present, no-op.
	if intakeCol.Fields.GetByName("site") != nil {
		return nil
	}

	sitesCol, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		return err
	}
	eventsCol, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}

	// Capture each intake's old event value before the field is removed.
	recs, err := app.FindRecordsByFilter(intakeCol.Id, "", "", 100000, 0)
	if err != nil {
		return err
	}
	oldEvents := make(map[string]string, len(recs))
	for _, rec := range recs {
		oldEvents[rec.Id] = rec.GetString("event")
	}

	// Remove the event field and re-add site as an optional single-select
	// relation to sites.
	intakeCol.Fields.RemoveByName("event")
	intakeCol.Fields.Add(&core.RelationField{
		Name:          "site",
		CollectionId:  sitesCol.Id,
		Required:      false,
		MaxSelect:     1,
		CascadeDelete: false,
	})
	if err := app.Save(intakeCol); err != nil {
		return err
	}

	// Backfill (best-effort): restore site from each intake's event's site.
	recs, err = app.FindRecordsByFilter(intakeCol.Id, "", "", 100000, 0)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		oldEvent := oldEvents[rec.Id]
		if oldEvent == "" {
			continue
		}
		ev, err := app.FindRecordById(eventsCol.Id, oldEvent)
		if err != nil {
			// Best-effort: leave site unset (optional field).
			continue
		}
		if ev.GetString("site") == "" {
			continue
		}
		rec.Set("site", ev.GetString("site"))
		if err := app.Save(rec); err != nil {
			return err
		}
	}

	return nil
}
