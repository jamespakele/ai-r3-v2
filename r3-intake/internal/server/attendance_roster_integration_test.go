package server

import (
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// rosterFixtures holds the record ids created by seedRosterData so tests can
// reference them.
type rosterFixtures struct {
	site, site2, ev1, ev2, cm, iInSite1, iInSite2, iOtherSite, iAssignedCM string
}

// seedRosterData creates two sites, two events, an admin user, a case manager
// user, and four intakes: two in the primary site, one in a second site, and
// one assigned to the case manager in the primary site.
func seedRosterData(t *testing.T, pb *pocketbase.PocketBase) rosterFixtures {
	t.Helper()
	save := func(name string, rec *core.Record) string {
		t.Helper()
		if err := pb.Save(rec); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return rec.Id
	}
	rec := func(name string) *core.Record {
		col, err := pb.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %s: %v", name, err)
		}
		return core.NewRecord(col)
	}

	site := save("site", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Kona")
		r.Set("active", true)
		return r
	}())
	site2 := save("site2", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Hilo")
		r.Set("active", true)
		return r
	}())

	ev := func(site, name string) string {
		return save(name, func() *core.Record {
			r := rec("events")
			r.Set("site", site)
			r.Set("name", name)
			r.Set("start_date", "2026-08-01")
			r.Set("end_date", "2026-08-31")
			r.Set("status", "active")
			return r
		}())
	}
	ev1 := ev(site, "Morning Program")
	ev2 := ev(site, "Evening Session")

	cm := save("cm", func() *core.Record {
		r := rec("users")
		r.SetEmail("cm@example.com")
		r.SetPassword("cm-password")
		r.Set("name", "Case Manager")
		r.Set("role", "case_manager")
		return r
	}())

	iInSite1 := save("iInSite1", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Alice")
		r.Set("site", site)
		return r
	}())
	iInSite2 := save("iInSite2", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Bob")
		r.Set("site", site)
		return r
	}())
	iOtherSite := save("iOtherSite", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Carol")
		r.Set("site", site2)
		return r
	}())
	iAssignedCM := save("iAssignedCM", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Dana")
		r.Set("site", site)
		r.Set("assigned_to", cm)
		return r
	}())

	return rosterFixtures{site, site2, ev1, ev2, cm, iInSite1, iInSite2, iOtherSite, iAssignedCM}
}

// saveAttendance records one attendance row with the given fields.
func saveAttendance(t *testing.T, pb *pocketbase.PocketBase, intake, site, event, date, status string) {
	t.Helper()
	col, err := pb.FindCollectionByNameOrId("attendance")
	if err != nil {
		t.Fatalf("attendance collection: %v", err)
	}
	r := core.NewRecord(col)
	r.Set("intake", intake)
	r.Set("site", site)
	if event != "" {
		r.Set("event", event)
	}
	r.Set("date", date)
	r.Set("status", status)
	if err := pb.Save(r); err != nil {
		t.Fatalf("save attendance: %v", err)
	}
}

// saveEnrollment records one event_enrollment row.
func saveEnrollment(t *testing.T, pb *pocketbase.PocketBase, event, intake string) {
	t.Helper()
	col, err := pb.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		t.Fatalf("event_enrollment collection: %v", err)
	}
	r := core.NewRecord(col)
	r.Set("event", event)
	r.Set("intake", intake)
	if err := pb.Save(r); err != nil {
		t.Fatalf("save enrollment: %v", err)
	}
}

// cellStatus returns the status of the matrix cell for row IntakeID and date,
// or "" when no such row/cell exists.
func cellStatus(rows []MatrixRow, intakeID, date string) string {
	for _, row := range rows {
		if row.IntakeID != intakeID {
			continue
		}
		for _, c := range row.Cells {
			if c.Date == date {
				return c.Status
			}
		}
	}
	return ""
}

// TestMatrixRosterEventIndependent proves the roster is identical whether or
// not an event is selected: an admin sees the full intake roster in both
// cases. The attendance map is scoped to the selected event's records, so
// dots differ only for the attendee whose record belongs to a different
// event.
func TestMatrixRosterEventIndependent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	// Event ev1 enrollment for an in-site intake.
	saveEnrollment(t, srv.pb, fx.ev1, fx.iInSite1)

	// Attendance on 2026-08-13: in-site intakes present under ev1 and ev2, and
	// an out-of-site walk-in under ev1.
	saveAttendance(t, srv.pb, fx.iInSite1, fx.site, fx.ev1, "2026-08-13", "present")
	saveAttendance(t, srv.pb, fx.iInSite2, fx.site, fx.ev2, "2026-08-13", "present")
	saveAttendance(t, srv.pb, fx.iOtherSite, fx.site2, fx.ev1, "2026-08-13", "walk_in")

	admin := &sessionUser{ID: "admin1", Email: "admin@example.com", Name: "Admin One", Role: "admin"}
	dates := []string{"2026-08-13"}

	withEvent, err := srv.loadMatrixRows(admin, dates, fx.ev1, "2026-08-13")
	if err != nil {
		t.Fatalf("loadMatrixRows(ev1): %v", err)
	}
	noEvent, err := srv.loadMatrixRows(admin, dates, "", "2026-08-13")
	if err != nil {
		t.Fatalf("loadMatrixRows(no event): %v", err)
	}

	// Roster independence: the participant list is identical with or without
	// an event selected; an admin sees every intake in both cases.
	idsOf := func(rows []MatrixRow) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.IntakeID)
		}
		return out
	}
	want := []string{fx.iInSite1, fx.iInSite2, fx.iOtherSite, fx.iAssignedCM} // Alice, Bob, Carol, Dana
	if got := idsOf(withEvent); !equalStrings(got, want) {
		t.Errorf("roster with event = %v, want %v", got, want)
	}
	if got := idsOf(noEvent); !equalStrings(got, want) {
		t.Errorf("roster without event = %v, want %v", got, want)
	}

	// Attendance map scoping: ev1 call populates only ev1 records; "" call
	// populates all in-range records regardless of event.
	if got := cellStatus(withEvent, fx.iInSite1, "2026-08-13"); got != "present" {
		t.Errorf("with event: iInSite1 status = %q, want present", got)
	}
	if got := cellStatus(withEvent, fx.iInSite2, "2026-08-13"); got != "" {
		t.Errorf("with event: iInSite2 status = %q, want \"\" (its record is ev2, not ev1)", got)
	}
	if got := cellStatus(noEvent, fx.iInSite1, "2026-08-13"); got != "present" {
		t.Errorf("no event: iInSite1 status = %q, want present", got)
	}
	if got := cellStatus(noEvent, fx.iInSite2, "2026-08-13"); got != "present" {
		t.Errorf("no event: iInSite2 status = %q, want present (ev2 record in range)", got)
	}
}

// equalStrings compares two string slices element-wise.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMatrixNoEventAdminAllIntakes proves that with no event selected, an
// admin's matrix renders the full intake roster and surfaces all in-range
// attendance regardless of which event it was recorded under.
func TestMatrixNoEventAdminAllIntakes(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)

	admin := &sessionUser{ID: "admin1", Email: "admin@example.com", Name: "Admin One", Role: "admin"}
	dates := []string{"2026-08-01", "2026-08-02", "2026-08-10"}

	rows, err := srv.loadMatrixRows(admin, dates, "", "2026-08-10")
	if err != nil {
		t.Fatalf("loadMatrixRows: %v", err)
	}

	// Both intakes render: i1 (Kona) and i2 (Waianae).
	found := map[string]bool{}
	for _, r := range rows {
		found[r.IntakeID] = true
	}
	if !found[fx.i1] || !found[fx.i2] {
		t.Errorf("roster = %v, want both i1 and i2 (admin, no event)", found)
	}

	// All in-range records surface regardless of event: att-1/att-2 (ev1) and
	// att-5 (ev2) for i1; att-3 (ev2) for i2.
	if got := cellStatus(rows, fx.i1, "2026-08-01"); got != "present" {
		t.Errorf("i1 08-01 status = %q, want present (att-1 via ev1)", got)
	}
	if got := cellStatus(rows, fx.i1, "2026-08-02"); got != "walk_in" {
		t.Errorf("i1 08-02 status = %q, want walk_in (att-2 via ev1)", got)
	}
	if got := cellStatus(rows, fx.i1, "2026-08-10"); got != "present" {
		t.Errorf("i1 08-10 status = %q, want present (att-5 via ev2)", got)
	}
	if got := cellStatus(rows, fx.i2, "2026-08-02"); got != "absent" {
		t.Errorf("i2 08-02 status = %q, want absent (att-3 via ev2)", got)
	}
}

// TestMatrixNoEventEmptyCells proves that with no event selected and no
// attendance records, the full roster still renders with empty cells.
func TestMatrixNoEventEmptyCells(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	admin := &sessionUser{ID: "admin1", Email: "admin@example.com", Name: "Admin One", Role: "admin"}
	dates := []string{"2026-08-13"}

	rows, err := srv.loadMatrixRows(admin, dates, "", "2026-08-13")
	if err != nil {
		t.Fatalf("loadMatrixRows: %v", err)
	}

	found := false
	for _, r := range rows {
		if r.IntakeID == fx.iOtherSite {
			found = true
		}
	}
	if !found {
		t.Fatalf("roster missing iOtherSite (full roster must render with no event selected)")
	}
	if got := cellStatus(rows, fx.iOtherSite, "2026-08-13"); got != "" {
		t.Errorf("iOtherSite status = %q, want \"\" (no attendance records)", got)
	}
}

// TestAttendanceSchemaNoSiteField proves migration 015 removed the site field
// from the attendance collection schema.
func TestAttendanceSchemaNoSiteField(t *testing.T) {
	srv := newTestServer(t)

	attCol, err := srv.pb.FindCollectionByNameOrId("attendance")
	if err != nil {
		t.Fatalf("attendance collection: %v", err)
	}
	if f := attCol.Fields.GetByName("site"); f != nil {
		t.Errorf("attendance schema still has a site field; migration 015 should have removed it")
	}
}
