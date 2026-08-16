package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// attJoinFixtures holds the record ids created by seedAttJoinFixtures so tests
// can reference them for filter assertions. Names are prefixed with attJoin to
// avoid colliding with the sibling records_list_integration_test.go helpers
// (listFixtures/seedListFixtures/...) when the epic merges all child worktrees.
type attJoinFixtures struct {
	site1, ev1, ev2, admin1, intakeA, intakeB, intakeC string
}

// seedAttJoinFixtures creates one site, two events, one admin user, and three
// intakes with attendance records covering the event-filter union edge cases:
//   - intakeA: home ev1, attended ev2 (cross-event; surfaces via attendance).
//   - intakeB: home ev2, attended ev2 (home-event AND attendance both match).
//   - intakeC: home ev1, attended ev1 (home-event AND attendance both match).
//
// Distinct names (Alice/Bob/Charlie) and statuses (claimed/unassigned/completed)
// let tests assert search and status composition.
func seedAttJoinFixtures(t *testing.T, pb *pocketbase.PocketBase) attJoinFixtures {
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

	site1 := save("site1", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Kona")
		r.Set("active", true)
		return r
	}())

	ev1 := save("ev1", func() *core.Record {
		r := rec("events")
		r.Set("site", site1)
		r.Set("name", "Morning Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())
	ev2 := save("ev2", func() *core.Record {
		r := rec("events")
		r.Set("site", site1)
		r.Set("name", "Job Fair")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())

	admin1 := save("admin1", func() *core.Record {
		r := rec("users")
		r.SetEmail("admin@example.com")
		r.SetPassword("admin-password")
		r.Set("name", "Admin One")
		r.Set("role", "admin")
		return r
	}())

	intakeA := save("intakeA", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Alice")
		r.Set("event", ev1)
		r.Set("status", "claimed")
		return r
	}())
	intakeB := save("intakeB", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Bob")
		r.Set("event", ev2)
		r.Set("status", "unassigned")
		return r
	}())
	intakeC := save("intakeC", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Charlie")
		r.Set("event", ev1)
		r.Set("status", "completed")
		return r
	}())

	// Attendance for ev2: intakeA (cross-event) and intakeB (home-event too).
	save("attA-ev2", func() *core.Record {
		r := rec("attendance")
		r.Set("event", ev2)
		r.Set("intake", intakeA)
		r.Set("date", "2026-08-01")
		r.Set("status", "present")
		return r
	}())
	save("attB-ev2", func() *core.Record {
		r := rec("attendance")
		r.Set("event", ev2)
		r.Set("intake", intakeB)
		r.Set("date", "2026-08-02")
		r.Set("status", "present")
		return r
	}())
	// Attendance for ev1: intakeC (home-event too).
	save("attC-ev1", func() *core.Record {
		r := rec("attendance")
		r.Set("event", ev1)
		r.Set("intake", intakeC)
		r.Set("date", "2026-08-03")
		r.Set("status", "present")
		return r
	}())

	return attJoinFixtures{site1, ev1, ev2, admin1, intakeA, intakeB, intakeC}
}

// doAttJoinList performs an authenticated GET on the records list screen with
// the given query string (e.g. "?event=<id>&q=Al").
func doAttJoinList(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/"+query, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// attJoinRowNames scans the rendered list for every `class="admin-link">NAME</a>`
// row and returns the names found, in body order. Each name in the variadic
// `names` argument must be present or the test fails. The returned slice lets
// callers assert exact row count (no duplicates, no extras).
func attJoinRowNames(t *testing.T, rec *httptest.ResponseRecorder, names ...string) []string {
	t.Helper()
	body := rec.Body.String()
	var found []string
	rest := body
	for {
		idx := strings.Index(rest, `class="admin-link">`)
		if idx < 0 {
			break
		}
		rest = rest[idx+len(`class="admin-link">`):]
		end := strings.Index(rest, "</a>")
		if end < 0 {
			break
		}
		found = append(found, rest[:end])
		rest = rest[end+len("</a>"):]
	}
	for _, want := range names {
		if !containsString(found, want) {
			t.Errorf("row %q not found in list; got %v", want, found)
		}
	}
	return found
}

// attJoinCount extracts the rendered result-count text, e.g. "Showing 2 records".
func attJoinCount(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := rec.Body.String()
	idx := strings.Index(body, `class="admin-result-count">`)
	if idx < 0 {
		t.Fatalf("admin-result-count not found in response")
	}
	rest := body[idx+len(`class="admin-result-count">`):]
	end := strings.Index(rest, "</p>")
	if end < 0 {
		t.Fatalf("closing </p> not found after admin-result-count")
	}
	return rest[:end]
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestListEventFilterDedupsHomeAndAttendance proves an intake matching the
// selected event via BOTH its home event and an attendance record is returned
// exactly once (the union must not duplicate rows).
func TestListEventFilterDedupsHomeAndAttendance(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	// ev2: intakeB matches via home event AND attendance; must appear once.
	rec := doAttJoinList(srv, cookie, "?event="+fx.ev2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Bob")
	if len(got) != 2 {
		t.Fatalf("ev2 rows = %v, want exactly [Alice Bob]", got)
	}
	if count := attJoinCount(t, rec); count != "Showing 2 records" {
		t.Errorf("ev2 count = %q, want %q", count, "Showing 2 records")
	}

	// ev1: intakeC matches via home event AND attendance; must appear once.
	rec = doAttJoinList(srv, cookie, "?event="+fx.ev1)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got = attJoinRowNames(t, rec, "Charlie")
	if len(got) != 2 {
		t.Fatalf("ev1 rows = %v, want exactly [Alice Charlie]", got)
	}
	if count := attJoinCount(t, rec); count != "Showing 2 records" {
		t.Errorf("ev1 count = %q, want %q", count, "Showing 2 records")
	}
}

// TestListEventFilterSurfacesMultipleAttendees proves the union scales to N
// attendees: both intakes with ev2 attendance surface, and intakeB's home-event
// match does not duplicate it.
func TestListEventFilterSurfacesMultipleAttendees(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	rec := doAttJoinList(srv, cookie, "?event="+fx.ev2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Alice", "Bob")
	if len(got) != 2 {
		t.Fatalf("ev2 rows = %v, want exactly [Alice Bob]", got)
	}
	if count := attJoinCount(t, rec); count != "Showing 2 records" {
		t.Errorf("ev2 count = %q, want %q", count, "Showing 2 records")
	}
}

// TestListEventFilterComposesWithSearch proves the event union is &&-composed
// with the ?q= free-text search: only the attendee whose name matches remains.
func TestListEventFilterComposesWithSearch(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	rec := doAttJoinList(srv, cookie, "?event="+fx.ev2+"&q=Al")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Alice")
	if len(got) != 1 {
		t.Fatalf("rows = %v, want exactly [Alice]", got)
	}
	if count := attJoinCount(t, rec); count != `Showing 1 record matching "Al"` {
		t.Errorf("count = %q, want %q", count, `Showing 1 record matching "Al"`)
	}
}

// TestListEventFilterComposesWithStatusAndSearch proves the three-way &&:
// event union, status filter, and free-text search all apply together.
func TestListEventFilterComposesWithStatusAndSearch(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	rec := doAttJoinList(srv, cookie, "?event="+fx.ev2+"&status=claimed&q=Al")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Alice")
	if len(got) != 1 {
		t.Fatalf("rows = %v, want exactly [Alice]", got)
	}
	if count := attJoinCount(t, rec); count != `Showing 1 record matching "Al"` {
		t.Errorf("count = %q, want %q", count, `Showing 1 record matching "Al"`)
	}
}

// TestListEventFilterCrossEventDistinct proves an intake surfaces for an event
// it attended even when its home event is a different event, while an intake
// with no attendance for the selected event does not.
func TestListEventFilterCrossEventDistinct(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	rec := doAttJoinList(srv, cookie, "?event="+fx.ev2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Alice", "Bob")
	if len(got) != 2 {
		t.Fatalf("ev2 rows = %v, want exactly [Alice Bob]", got)
	}
	for _, name := range got {
		if name == "Charlie" {
			t.Errorf("Charlie (home ev1, no ev2 attendance) must not surface for ev2; got %v", got)
		}
	}
}

// TestListNoEventFilterReturnsAll is a regression guard: with no event filter
// the list returns every intake, proving the join code path does not break the
// unfiltered list.
func TestListNoEventFilterReturnsAll(t *testing.T) {
	srv := newTestServer(t)
	fx := seedAttJoinFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	rec := doAttJoinList(srv, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := attJoinRowNames(t, rec, "Alice", "Bob", "Charlie")
	if len(got) != 3 {
		t.Fatalf("rows = %v, want exactly [Alice Bob Charlie]", got)
	}
	if count := attJoinCount(t, rec); count != "Showing 3 records" {
		t.Errorf("count = %q, want %q", count, "Showing 3 records")
	}
}
