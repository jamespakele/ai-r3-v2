package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// eventDefaultFixtures holds the record ids created by seedEventDefaultFixtures
// so tests can reference them for assertions.
type eventDefaultFixtures struct {
	ev1, ev2, admin1, intakeNoEvent, intakeEv2 string
}

// seedEventDefaultFixtures creates a site, two active events (ev1 starts
// before ev2, so loadEvents' start_date,name sort puts ev1 first), an admin
// user, and two intakes: one with no event set, one whose event is ev2.
func seedEventDefaultFixtures(t *testing.T, pb *pocketbase.PocketBase) eventDefaultFixtures {
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
		r.Set("start_date", "2026-09-01")
		r.Set("end_date", "2026-09-30")
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

	intakeNoEvent := save("intakeNoEvent", func() *core.Record {
		r := rec("intake")
		r.Set("name", "No Event")
		r.Set("status", "unassigned")
		return r
	}())
	intakeEv2 := save("intakeEv2", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Has Event")
		r.Set("event", ev2)
		r.Set("status", "unassigned")
		return r
	}())

	return eventDefaultFixtures{ev1, ev2, admin1, intakeNoEvent, intakeEv2}
}

// doIntakeEdit issues GET /intake/{id} with the given cookie.
func doIntakeEdit(srv *Server, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/intake/"+id, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestIntakeEditDefaultsEventToFirstActive proves an intake record with no
// stored event renders the first active event selected in the dropdown, and
// that a record with a stored event keeps its own event selected.
func TestIntakeEditDefaultsEventToFirstActive(t *testing.T) {
	srv := newTestServer(t)
	fx := seedEventDefaultFixtures(t, srv.pb)
	cookie := adminCookie(srv, fx.admin1)

	t.Run("empty event defaults to first active", func(t *testing.T) {
		rec := doIntakeEdit(srv, cookie, fx.intakeNoEvent)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /intake/%s: status %d, want 200", fx.intakeNoEvent, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<option value="`+fx.ev1+`" selected>`) {
			t.Errorf("first active event %s not rendered selected:\n%s", fx.ev1, body)
		}
		if strings.Contains(body, `<option value="`+fx.ev2+`" selected>`) {
			t.Errorf("second event %s unexpectedly rendered selected:\n%s", fx.ev2, body)
		}
	})

	t.Run("stored event stays selected", func(t *testing.T) {
		rec := doIntakeEdit(srv, cookie, fx.intakeEv2)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /intake/%s: status %d, want 200", fx.intakeEv2, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<option value="`+fx.ev2+`" selected>`) {
			t.Errorf("stored event %s not rendered selected:\n%s", fx.ev2, body)
		}
		if strings.Contains(body, `<option value="`+fx.ev1+`" selected>`) {
			t.Errorf("first event %s unexpectedly rendered selected:\n%s", fx.ev1, body)
		}
	})
}
