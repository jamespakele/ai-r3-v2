package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// upAttendanceEventRequired backfills attendance records whose event relation
// is null into a synthetic per-site "Legacy / Unassigned" event, then flips
// the attendance.event relation field to required so all future attendance is
// event-scoped. Uniqueness is keyed on (event, intake, date).
func upAttendanceEventRequired(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}

	// Idempotency guard: if the event field is already required, this
	// migration has already run — no-op.
	f := attCol.Fields.GetByName("event")
	if rf, ok := f.(*core.RelationField); ok && rf.Required {
		return nil
	}

	// Load all null-event attendance records. attendance.date is required, so
	// every record here has a valid date to derive legacy event bounds.
	recs, err := app.FindRecordsByFilter(attCol.Id, "event=''", "date", 100000, 0)
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		// Nothing to backfill; still need to flip the field to required.
		return flipEventRequired(app, attCol)
	}

	// Group null-event records by their site (may be "" for no-location
	// participants, per migration 009).
	bySite := map[string][]*core.Record{}
	for _, rec := range recs {
		site := rec.GetString("site")
		bySite[site] = append(bySite[site], rec)
	}

	for siteID, group := range bySite {
		legacySite, err := resolveLegacyEventSite(app, siteID, group)
		if err != nil {
			return err
		}
		if legacySite == "" {
			// Use the default site, or create a dedicated fallback site so no
			// attendance record is left with a dangling required event.
			legacySite, err = fallbackLegacySite(app)
			if err != nil {
				return err
			}
		}
		minDate, maxDate := minMaxDates(group)
		legacy, err := createLegacyEvent(app, legacySite, minDate, maxDate)
		if err != nil {
			return err
		}
		for _, rec := range group {
			rec.Set("event", legacy.Id)
			if err := app.Save(rec); err != nil {
				return err
			}
		}
	}

	return flipEventRequired(app, attCol)
}

// flipEventRequired marks attendance.event as required and persists the
// collection schema.
func flipEventRequired(app core.App, attCol *core.Collection) error {
	f := attCol.Fields.GetByName("event")
	if rf, ok := f.(*core.RelationField); ok && !rf.Required {
		rf.Required = true
		return app.Save(attCol)
	}
	return nil
}

// resolveLegacyEventSite returns the site id to assign to a legacy event for
// a group of null-event attendance records. When the group's own site is
// empty, it falls back to the site of the first record's linked intake. It
// returns "" if no site can be determined (caller leaves records unassigned).
func resolveLegacyEventSite(app core.App, siteID string, group []*core.Record) (string, error) {
	if siteID != "" {
		return siteID, nil
	}
	intakeCol, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return "", err
	}
	for _, rec := range group {
		iid := rec.GetString("intake")
		if iid == "" {
			continue
		}
		intakeRec, err := app.FindRecordById(intakeCol.Id, iid)
		if err != nil {
			continue
		}
		if sid := intakeRec.GetString("site"); sid != "" {
			return sid, nil
		}
	}
	return "", nil
}

// minMaxDates computes the lexicographic (YYYY-MM-DD) min and max dates across
// a group of attendance records. Falls back to "" for both when none parse.
func minMaxDates(group []*core.Record) (string, string) {
	minDate, maxDate := "", ""
	for _, rec := range group {
		d := rec.GetString("date")
		if d == "" {
			continue
		}
		if minDate == "" || d < minDate {
			minDate = d
		}
		if maxDate == "" || d > maxDate {
			maxDate = d
		}
	}
	return minDate, maxDate
}

// createLegacyEvent creates one synthetic "Legacy / Unassigned" event for a
// site and returns it.
func createLegacyEvent(app core.App, siteID, minDate, maxDate string) (*core.Record, error) {
	eventsCol, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return nil, err
	}
	legacy := core.NewRecord(eventsCol)
	legacy.Set("site", siteID)
	legacy.Set("name", "Legacy / Unassigned")
	legacy.Set("start_date", minDate)
	legacy.Set("end_date", maxDate)
	legacy.Set("status", "active")
	legacy.Set("description", "Auto-created by migration 010 to preserve pre-event-scoped attendance records.")
	// created_by intentionally left unset.
	if err := app.Save(legacy); err != nil {
		return nil, err
	}
	return legacy, nil
}

// fallbackLegacySite returns the id of the default site, creating a dedicated
// "Unassigned" site if no default exists.
func fallbackLegacySite(app core.App) (string, error) {
	sitesCol, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		return "", err
	}
	recs, err := app.FindRecordsByFilter(sitesCol.Id, "is_default=true", "", 1, 0)
	if err != nil {
		return "", err
	}
	if len(recs) > 0 {
		return recs[0].Id, nil
	}
	fallback := core.NewRecord(sitesCol)
	fallback.Set("name", "Unassigned")
	fallback.Set("active", true)
	fallback.Set("sort_order", 9999)
	if err := app.Save(fallback); err != nil {
		return "", err
	}
	return fallback.Id, nil
}

// downAttendanceEventRequired flips attendance.event back to optional.
// Best-effort: it intentionally does NOT delete the synthetic legacy events,
// because attendance.event has cascadeDelete enabled — deleting a legacy event
// would cascade-delete the attendance records it now owns.
func downAttendanceEventRequired(app core.App) error {
	attCol, err := app.FindCollectionByNameOrId("attendance")
	if err != nil {
		return err
	}
	f := attCol.Fields.GetByName("event")
	if rf, ok := f.(*core.RelationField); ok && rf.Required {
		rf.Required = false
		return app.Save(attCol)
	}
	return nil
}
