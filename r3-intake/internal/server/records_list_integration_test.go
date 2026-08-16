package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// listFixtures holds the record ids created by seedListFixtures so tests can
// reference them for filter assertions.
type listFixtures struct {
	site1, ev1, ev2, admin1, intakeA, intakeB string
}

// seedListFixtures creates a site, two events, an admin user, and two intakes:
// intakeA's home event is ev1 but it has an attendance record for ev2
// (analogous to the verification scenario: home event differs from the
// attended event); intakeB's home event is ev2. intakeA is claimed, intakeB
// is unassigned.
func seedListFixtures(t *testing.T, pb *pocketbase.PocketBase) listFixtures {
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
		r.Set("name", "Event One")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())
	ev2 := save("ev2", func() *core.Record {
		r := rec("events")
		r.Set("site", site1)
		r.Set("name", "Event Two")
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

	// Alice attended ev2 even though her home event is ev1.
	save("att-alice-ev2", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", intakeA)
		r.Set("event", ev2)
		r.Set("date", "2026-08-01")
		r.Set("status", "present")
		return r
	}())

	return listFixtures{site1, ev1, ev2, admin1, intakeA, intakeB}
}

// doList issues GET / with the given query string and cookie.
func doList(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/"+query, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// listRowNames returns the participant names rendered as admin-link rows.
func listRowNames(t *testing.T, rec *httptest.ResponseRecorder, names ...string) []string {
	t.Helper()
	body := rec.Body.String()
	found := []string{}
	for _, n := range names {
		if strings.Contains(body, `class="admin-link">`+n+`</a>`) {
			found = append(found, n)
		}
	}
	return found
}

// listCount extracts the "Showing N record(s)" total from the rendered page.
func listCount(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := rec.Body.String()
	idx := strings.Index(body, `class="admin-result-count">`)
	if idx < 0 {
		t.Fatalf("no result count in body: %s", body)
	}
	rest := body[idx+len(`class="admin-result-count">`):]
	end := strings.Index(rest, "</p>")
	if end < 0 {
		t.Fatalf("unterminated result count in body: %s", body)
	}
	return rest[:end]
}

// TestListEventFilterJoinsAttendance proves the Records event filter is a
// union: an intake matches when its home event equals the selected event OR
// it has an attendance record for that event (attendance.intake == intake.id).
// It also verifies the union composes with the status filter and that the
// rendered total matches the returned rows.
func TestListEventFilterJoinsAttendance(t *testing.T) {
	srv := newTestServer(t)
	fx := seedListFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	t.Run("event with attendance from other-home-event intake", func(t *testing.T) {
		rec := doList(srv, cookie, "?event="+fx.ev2)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// Alice (home ev1, attended ev2) surfaces via the attendance join;
		// Bob (home ev2) surfaces via the home-event branch.
		got := listRowNames(t, rec, "Alice", "Bob")
		if len(got) != 2 {
			t.Fatalf("rows = %v, want [Alice Bob]", got)
		}
		if count := listCount(t, rec); !strings.Contains(count, "Showing 2 records") {
			t.Errorf("count = %q, want %q", count, "Showing 2 records")
		}
	})

	t.Run("event with no cross-event attendance", func(t *testing.T) {
		rec := doList(srv, cookie, "?event="+fx.ev1)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// Alice's home event is ev1; Bob has no attendance for ev1.
		got := listRowNames(t, rec, "Alice", "Bob")
		if len(got) != 1 || got[0] != "Alice" {
			t.Fatalf("rows = %v, want [Alice]", got)
		}
		if count := listCount(t, rec); !strings.Contains(count, "Showing 1 record") {
			t.Errorf("count = %q, want %q", count, "Showing 1 record")
		}
	})

	t.Run("union composes with status filter", func(t *testing.T) {
		rec := doList(srv, cookie, "?event="+fx.ev2+"&status=claimed")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// Only Alice is claimed; Bob is unassigned.
		got := listRowNames(t, rec, "Alice", "Bob")
		if len(got) != 1 || got[0] != "Alice" {
			t.Fatalf("rows = %v, want [Alice]", got)
		}
		if count := listCount(t, rec); !strings.Contains(count, "Showing 1 record") {
			t.Errorf("count = %q, want %q", count, "Showing 1 record")
		}
	})

	t.Run("event with no attendance falls back to home event", func(t *testing.T) {
		// ev1 has no attendance records at all; the filter must still return
		// intakes whose home event matches (no empty-screen regression).
		rec := doList(srv, cookie, "?event="+fx.ev1)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := listRowNames(t, rec, "Alice", "Bob")
		if len(got) != 1 || got[0] != "Alice" {
			t.Fatalf("rows = %v, want [Alice]", got)
		}
	})
}
