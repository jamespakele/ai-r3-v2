package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upAttendanceRemoveSite removes the redundant attendance.site relation field.
// Since migration 010 made attendance.event required and events.site is
// required, the event's site is the single authoritative source of location
// for every attendance row — the stored attendance.site value is redundant.
//
// The migration performs a non-blocking data-integrity backfill check: if any
// row's stored site differs from its event's site, a warning is logged but the
// migration proceeds (the event's site is authoritative and the divergent
// stored value is discarded). Removing the field does not affect the unique
// index idx_attendance_event_intake_date, which does not reference site.
func upAttendanceRemoveSite(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}

	// Idempotency guard: if the site field is already absent, no-op.
	if attCol.Fields.GetByName("site") == nil {
		return nil
	}

	// Load the events collection once for the backfill safety check. If it
	// cannot be resolved, log a warning and skip the check — the field is
	// being removed regardless.
	eventsCol, eventsErr := app.FindCollectionByNameOrId("events")
	if eventsErr != nil {
		app.Logger().Warn("attendance-remove-site: could not load events collection for backfill check", "error", eventsErr)
	} else {
		recs, err := app.FindRecordsByFilter(attCol.Id, "", "", 100000, 0)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			ev, err := app.FindRecordById(eventsCol.Id, rec.GetString("event"))
			if err != nil {
				app.Logger().Warn("attendance-remove-site: could not resolve event for backfill check", "record_id", rec.Id, "error", err)
				continue
			}
			storedSite := rec.GetString("site")
			eventSite := ev.GetString("site")
			if storedSite != eventSite {
				app.Logger().Warn(
					"attendance-remove-site: stored site diverges from event site; event site is authoritative",
					"record_id", rec.Id,
					"stored_site", storedSite,
					"event_site", eventSite,
				)
			}
		}
	}

	// Remove the redundant field and persist the schema.
	attCol.Fields.RemoveByName("site")
	if err := app.Save(attCol); err != nil {
		return err
	}

	return nil
}

// downAttendanceRemoveSite restores attendance.site as an optional relation to
// sites (matching the post-009 state) and backfills each row's site from its
// event's site so the rollback is lossless. Backfill is best-effort: rows
// whose event cannot be resolved or whose event has no site keep site empty
// (it is optional).
func downAttendanceRemoveSite(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}

	// Idempotency guard: if the site field is already present, no-op.
	if attCol.Fields.GetByName("site") != nil {
		return nil
	}

	sitesCol, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		return err
	}

	// Re-add the field as an optional single-select relation to sites.
	attCol.Fields.Add(&core.RelationField{
		Name:          "site",
		CollectionId:  sitesCol.Id,
		Required:      false,
		MaxSelect:     1,
		CascadeDelete: false,
	})
	if err := app.Save(attCol); err != nil {
		return err
	}

	// Backfill from each row's event's site. Schema is already altered; a
	// failure to resolve events here is fatal before backfill proceeds.
	eventsCol, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}

	recs, err := app.FindRecordsByFilter(attCol.Id, "", "", 100000, 0)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		ev, err := app.FindRecordById(eventsCol.Id, rec.GetString("event"))
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
