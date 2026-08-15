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

// TestMatrixRosterEventIndependent proves AC #1: loadMatrixRows returns the
// identical ordered participant roster whether or not an event is selected.
// The selected event only scopes the attendance map, never the roster.
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

	withEvent, err := srv.loadMatrixRows(admin, fx.site, dates, fx.ev1, "2026-08-13")
	if err != nil {
		t.Fatalf("loadMatrixRows(ev1): %v", err)
	}
	noEvent, err := srv.loadMatrixRows(admin, fx.site, dates, "", "2026-08-13")
	if err != nil {
		t.Fatalf("loadMatrixRows(no event): %v", err)
	}

	// Roster identity: identical ordered IntakeIDs (full site-scoped roster).
	idsOf := func(rows []MatrixRow) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.IntakeID)
		}
		return out
	}
	want := []string{fx.iInSite1, fx.iInSite2, fx.iAssignedCM} // Alice, Bob, Dana
	if got := idsOf(withEvent); !equalStrings(got, want) {
		t.Errorf("roster with event = %v, want %v", got, want)
	}
	if got := idsOf(noEvent); !equalStrings(got, want) {
		t.Errorf("roster without event = %v, want %v", got, want)
	}

	// Out-of-site walk-in is recorded but never rendered as a matrix row.
	for _, r := range withEvent {
		if r.IntakeID == fx.iOtherSite {
			t.Errorf("out-of-site walk-in intake rendered as a row")
		}
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

// TestMatrixSitedFilteringAllEvents proves that a site-scoped matrix with no
// selected event resolves the site's active event set and excludes attendance
// recorded under events at other sites, while still surfacing same-site
// records across all of that site's events.
func TestMatrixSitedFilteringAllEvents(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)

	admin := &sessionUser{ID: "admin1", Email: "admin@example.com", Name: "Admin One", Role: "admin"}
	dates := []string{"2026-08-01", "2026-08-02", "2026-08-10"}

	rows, err := srv.loadMatrixRows(admin, fx.site1, dates, "", "2026-08-10")
	if err != nil {
		t.Fatalf("loadMatrixRows: %v", err)
	}

	// Same-site records under ev1 (Kona) surface in the site-scoped matrix.
	if got := cellStatus(rows, fx.i1, "2026-08-01"); got != "present" {
		t.Errorf("i1 08-01 status = %q, want present (att-1 via ev1)", got)
	}
	if got := cellStatus(rows, fx.i1, "2026-08-02"); got != "walk_in" {
		t.Errorf("i1 08-02 status = %q, want walk_in (att-2 via ev1)", got)
	}
	// att-5 is i1 attending ev2 (Waianae); it must NOT surface in the Kona
	// site-scoped matrix.
	if got := cellStatus(rows, fx.i1, "2026-08-10"); got != "" {
		t.Errorf("i1 08-10 status = %q, want \"\" (att-5 is under ev2/Waianae, excluded)", got)
	}
	// i2 is a Waianae intake; excluded from the Kona roster entirely.
	for _, r := range rows {
		if r.IntakeID == fx.i2 {
			t.Errorf("Waianae intake i2 rendered as a row in the Kona matrix")
		}
	}
}

// TestMatrixSiteNoActiveEvents proves a site with no active events still
// renders its full roster, but the empty resolved event set skips the
// attendance query so no cells are filled.
func TestMatrixSiteNoActiveEvents(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	admin := &sessionUser{ID: "admin1", Email: "admin@example.com", Name: "Admin One", Role: "admin"}
	dates := []string{"2026-08-13"}

	rows, err := srv.loadMatrixRows(admin, fx.site2, dates, "", "2026-08-13")
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
		t.Fatalf("site2 roster missing iOtherSite (roster must still render with no active events)")
	}
	if got := cellStatus(rows, fx.iOtherSite, "2026-08-13"); got != "" {
		t.Errorf("iOtherSite status = %q, want \"\" (empty event set skips attendance query)", got)
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
