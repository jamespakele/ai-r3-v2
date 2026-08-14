package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// doEventUpdate POSTs the Update Event form to /admin/events/{id}/update with
// the given form values, mirroring doEventCreate.
func doEventUpdate(srv *Server, cookie *http.Cookie, id string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/events/"+id+"/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// doEventDelete POSTs the soft-delete request to /admin/events/{id}/delete.
func doEventDelete(srv *Server, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/events/"+id+"/delete", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// markEventDeleted flips the deleted flag on the named event and re-saves it.
func markEventDeleted(t *testing.T, srv *Server, name string) {
	t.Helper()
	rec := findEventByName(t, srv, name)
	if rec == nil {
		t.Fatalf("event %q not found", name)
	}
	rec.Set("deleted", true)
	if err := srv.pb.Save(rec); err != nil {
		t.Fatalf("mark event %q deleted: %v", name, err)
	}
}

// TestAdminEventUpdateSuccess proves a valid update mutates only the editable
// fields, preserves lifecycle/ownership fields, and 303-redirects.
func TestAdminEventUpdateSuccess(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	// Give the event an explicit owner so we can prove update leaves it intact.
	ev0 := findEventByName(t, srv, "Morning Program")
	ev0.Set("created_by", fx.admin1)
	if err := srv.pb.Save(ev0); err != nil {
		t.Fatalf("set created_by: %v", err)
	}

	form := url.Values{
		"name":        {"Updated Event"},
		"site":        {fx.site},
		"start_date":  {"2026-08-15"},
		"end_date":    {"2026-08-20"},
		"description": {"updated desc"},
	}
	rec := doEventUpdate(srv, admin, fx.ev, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (event updated)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=events" {
		t.Errorf("Location = %q, want /admin?tab=events", loc)
	}

	ev := findEventByName(t, srv, "Updated Event")
	if ev == nil {
		t.Fatal("expected event 'Updated Event' after update, got none")
	}
	if got := ev.GetString("site"); got != fx.site {
		t.Errorf("site = %q, want %q", got, fx.site)
	}
	if got := ev.GetString("start_date"); got != "2026-08-15" {
		t.Errorf("start_date = %q, want 2026-08-15", got)
	}
	if got := ev.GetString("end_date"); got != "2026-08-20" {
		t.Errorf("end_date = %q, want 2026-08-20", got)
	}
	if got := ev.GetString("description"); got != "updated desc" {
		t.Errorf("description = %q, want updated desc", got)
	}
	if got := ev.GetString("status"); got != "active" {
		t.Errorf("status = %q, want active (unchanged)", got)
	}
	if got := ev.GetString("created_by"); got != fx.admin1 {
		t.Errorf("created_by = %q, want %q (unchanged)", got, fx.admin1)
	}
	if ev.GetBool("deleted") {
		t.Error("deleted = true, want false (update must not soft-delete)")
	}
}

// TestAdminEventUpdateValidation proves each validation failure re-renders the
// admin template with HTTP 200, the exact error message, and the submitted
// values preserved, while leaving the stored record untouched.
func TestAdminEventUpdateValidation(t *testing.T) {
	longDesc := strings.Repeat("x", 501)

	cases := []struct {
		name    string
		form    url.Values
		wantMsg string
	}{
		{
			name: "empty name",
			form: url.Values{
				"name":        {""},
				"site":        {"Kona"},
				"start_date":  {"2026-08-01"},
				"end_date":    {"2026-08-31"},
				"description": {"d"},
			},
			wantMsg: "Event name and location are required.",
		},
		{
			name: "empty site",
			form: url.Values{
				"name":        {"Morning Program"},
				"site":        {""},
				"start_date":  {"2026-08-01"},
				"end_date":    {"2026-08-31"},
				"description": {"d"},
			},
			wantMsg: "Event name and location are required.",
		},
		{
			name: "invalid start date",
			form: url.Values{
				"name":        {"Morning Program"},
				"site":        {"Kona"},
				"start_date":  {"not-a-date"},
				"end_date":    {"2026-08-31"},
				"description": {"d"},
			},
			wantMsg: "Start and end dates must be valid dates.",
		},
		{
			name: "invalid end date",
			form: url.Values{
				"name":        {"Morning Program"},
				"site":        {"Kona"},
				"start_date":  {"2026-08-01"},
				"end_date":    {"not-a-date"},
				"description": {"d"},
			},
			wantMsg: "Start and end dates must be valid dates.",
		},
		{
			name: "end before start",
			form: url.Values{
				"name":        {"Morning Program"},
				"site":        {"Kona"},
				"start_date":  {"2026-08-20"},
				"end_date":    {"2026-08-15"},
				"description": {"d"},
			},
			wantMsg: "End date must be on or after start date.",
		},
		{
			name: "description too long",
			form: url.Values{
				"name":        {"Morning Program"},
				"site":        {"Kona"},
				"start_date":  {"2026-08-01"},
				"end_date":    {"2026-08-31"},
				"description": {longDesc},
			},
			wantMsg: "Description must be 500 characters or fewer.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			fx := seedToggleData(t, srv.pb)
			admin := adminCookie(srv, fx.admin1)

			rec := doEventUpdate(srv, admin, fx.ev, tc.form)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (validation re-render)", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Errorf("body missing error %q", tc.wantMsg)
			}
			// Submitted values preserved in the re-rendered form.
			for _, v := range tc.form {
				if v[0] == "" {
					continue
				}
				if !strings.Contains(rec.Body.String(), v[0]) {
					t.Errorf("body missing submitted value %q", v[0])
				}
			}
			// Stored record unchanged.
			ev := findEventByName(t, srv, "Morning Program")
			if ev == nil {
				t.Fatal("expected original event to remain, got none")
			}
			if got := ev.GetString("name"); got != "Morning Program" {
				t.Errorf("name = %q, want Morning Program (unchanged)", got)
			}
			if got := ev.GetString("start_date"); got != "2026-08-01" {
				t.Errorf("start_date = %q, want 2026-08-01 (unchanged)", got)
			}
			if got := ev.GetString("end_date"); got != "2026-08-31" {
				t.Errorf("end_date = %q, want 2026-08-31 (unchanged)", got)
			}
		})
	}
}

// TestAdminEventUpdateNotFound proves update 404s on a missing id and on a
// soft-deleted event.
func TestAdminEventUpdateNotFound(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		srv := newTestServer(t)
		fx := seedToggleData(t, srv.pb)
		admin := adminCookie(srv, fx.admin1)
		form := url.Values{
			"name": {"X"}, "site": {"Kona"},
			"start_date": {"2026-08-01"}, "end_date": {"2026-08-31"},
		}
		rec := doEventUpdate(srv, admin, "nonexistent", form)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("soft-deleted event", func(t *testing.T) {
		srv := newTestServer(t)
		fx := seedToggleData(t, srv.pb)
		admin := adminCookie(srv, fx.admin1)
		markEventDeleted(t, srv, "Morning Program")
		form := url.Values{
			"name": {"X"}, "site": {"Kona"},
			"start_date": {"2026-08-01"}, "end_date": {"2026-08-31"},
		}
		rec := doEventUpdate(srv, admin, fx.ev, form)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (deleted event)", rec.Code)
		}
	})
}

// TestAdminEventDeleteSuccess proves soft-delete sets deleted=true and
// 303-redirects.
func TestAdminEventDeleteSuccess(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doEventDelete(srv, admin, fx.ev)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (event deleted)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=events" {
		t.Errorf("Location = %q, want /admin?tab=events", loc)
	}

	ev := findEventByName(t, srv, "Morning Program")
	if ev == nil {
		t.Fatal("expected event record to remain after soft-delete, got none")
	}
	if !ev.GetBool("deleted") {
		t.Error("deleted = false, want true")
	}
}

// TestAdminEventDeleteIdempotent proves deleting an already-deleted event is a
// no-op that still 303-redirects.
func TestAdminEventDeleteIdempotent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	if rec := doEventDelete(srv, admin, fx.ev); rec.Code != http.StatusSeeOther {
		t.Fatalf("first delete status = %d, want 303", rec.Code)
	}
	if rec := doEventDelete(srv, admin, fx.ev); rec.Code != http.StatusSeeOther {
		t.Fatalf("second delete status = %d, want 303 (idempotent)", rec.Code)
	}
	ev := findEventByName(t, srv, "Morning Program")
	if ev == nil {
		t.Fatal("expected event record to remain, got none")
	}
	if !ev.GetBool("deleted") {
		t.Error("deleted = false, want true")
	}
}

// TestAdminEventDeleteNotFound proves delete 404s on a missing id.
func TestAdminEventDeleteNotFound(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doEventDelete(srv, admin, "nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestLoadAllEventsExcludesDeleted proves the admin list view omits soft-deleted
// events.
func TestLoadAllEventsExcludesDeleted(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	markEventDeleted(t, srv, "Morning Program")

	rows, err := srv.loadAllEvents()
	if err != nil {
		t.Fatalf("loadAllEvents: %v", err)
	}
	for _, r := range rows {
		if r.ID == fx.ev {
			t.Errorf("deleted event %q present in loadAllEvents", r.Name)
		}
	}
}

// TestLoadEventsFiltering proves loadEvents returns only active, non-deleted
// events, optionally scoped to a site.
func TestLoadEventsFiltering(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)

	// A second, non-deleted event on the same site with status=completed.
	col, err := srv.pb.FindCollectionByNameOrId("events")
	if err != nil {
		t.Fatalf("events collection: %v", err)
	}
	completed := core.NewRecord(col)
	completed.Set("site", fx.site)
	completed.Set("name", "Completed Program")
	completed.Set("start_date", "2026-07-01")
	completed.Set("end_date", "2026-07-31")
	completed.Set("status", "completed")
	if err := srv.pb.Save(completed); err != nil {
		t.Fatalf("save completed event: %v", err)
	}

	// Mark the active event deleted.
	markEventDeleted(t, srv, "Morning Program")

	// Site-scoped: only active, non-deleted events for that site.
	scoped, err := srv.loadEvents(fx.site)
	if err != nil {
		t.Fatalf("loadEvents(site): %v", err)
	}
	for _, e := range scoped {
		if e.ID == fx.ev {
			t.Errorf("deleted event %q present in site-scoped loadEvents", e.Name)
		}
		if e.ID == completed.Id {
			t.Errorf("completed event %q present in site-scoped loadEvents", e.Name)
		}
	}

	// Unscoped: same exclusions.
	all, err := srv.loadEvents("")
	if err != nil {
		t.Fatalf("loadEvents(\"\"): %v", err)
	}
	for _, e := range all {
		if e.ID == fx.ev {
			t.Errorf("deleted event %q present in unscoped loadEvents", e.Name)
		}
		if e.ID == completed.Id {
			t.Errorf("completed event %q present in unscoped loadEvents", e.Name)
		}
	}
}

// TestAdminEventUpdateDeleteAuthBoundary proves unauthenticated and non-admin
// requests to update/delete are redirected to /login.
func TestAdminEventUpdateDeleteAuthBoundary(t *testing.T) {
	form := url.Values{
		"name": {"X"}, "site": {"Kona"},
		"start_date": {"2026-08-01"}, "end_date": {"2026-08-31"},
	}

	for _, tc := range []struct {
		name   string
		cookie func(*Server, string) *http.Cookie
	}{
		{"unauthenticated", nil},
		{"case manager", cmCookie},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			fx := seedToggleData(t, srv.pb)
			var cookie *http.Cookie
			if tc.cookie != nil {
				cookie = tc.cookie(srv, fx.admin1)
			}

			rec := doEventUpdate(srv, cookie, fx.ev, form)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("update status = %d, want 303 redirect to /login", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != "/login" {
				t.Errorf("update Location = %q, want /login", loc)
			}

			rec = doEventDelete(srv, cookie, fx.ev)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("delete status = %d, want 303 redirect to /login", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != "/login" {
				t.Errorf("delete Location = %q, want /login", loc)
			}

			// No mutation happened for the rejected requests.
			ev := findEventByName(t, srv, "Morning Program")
			if ev == nil {
				t.Fatal("expected event to remain, got none")
			}
			if ev.GetBool("deleted") {
				t.Error("deleted = true, want false (rejected delete must not mutate)")
			}
		})
	}
}

// TestAdminEventUpdateCSRFRejected proves a POST without the CSRF token is
// rejected by the middleware before reaching the handler.
func TestAdminEventUpdateCSRFRejected(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"name": {"X"}, "site": {"Kona"},
		"start_date": {"2026-08-01"}, "end_date": {"2026-08-31"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/events/"+fx.ev+"/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (missing CSRF token)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid or missing csrf token") {
		t.Errorf("body missing csrf error, got %q", rec.Body.String())
	}
}
