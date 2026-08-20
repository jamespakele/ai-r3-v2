package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// sectionFixture carries the record ids a section-save test needs: a valid
// event and a real admin user (the session user must exist as a users record
// so the created_by/assigned_to relations validate on save).
type sectionFixture struct {
	event string
	admin string
}

// seedActiveEvent creates a site, an active event, and an admin user; it
// returns the event id and the admin user id.
func seedActiveEvent(t *testing.T, pb *pocketbase.PocketBase) sectionFixture {
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

	event := save("event", func() *core.Record {
		r := rec("events")
		r.Set("site", site)
		r.Set("name", "Morning Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())

	admin := createUser(t, pb, "admin@example.com", "Admin One", "admin", "admin-password", false)

	return sectionFixture{event: event, admin: admin}
}

// doSectionPost posts to /section/{section} with CSRF and optional admin
// session, optionally flagged as an htmx request.
func doSectionPost(t *testing.T, srv *Server, cookie *http.Cookie, hx bool, section string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/section/"+section, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// doSection01Post posts to /section/01 with CSRF and optional admin session.
func doSection01Post(t *testing.T, srv *Server, cookie *http.Cookie, hx bool, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return doSectionPost(t, srv, cookie, hx, "01", form)
}

// countIntakeRecords returns the number of rows in the intake collection.
func countIntakeRecords(t *testing.T, srv *Server) int {
	t.Helper()
	col, err := srv.pb.FindCollectionByNameOrId("intake")
	if err != nil {
		t.Fatalf("intake collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id, "", "", 0, 0)
	if err != nil {
		t.Fatalf("find intake records: %v", err)
	}
	return len(recs)
}

// firstIntakeRecord returns the single intake record, failing if the count is
// not exactly one.
func firstIntakeRecord(t *testing.T, srv *Server) *core.Record {
	t.Helper()
	col, err := srv.pb.FindCollectionByNameOrId("intake")
	if err != nil {
		t.Fatalf("intake collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id, "", "", 0, 0)
	if err != nil {
		t.Fatalf("find intake records: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("intake records = %d, want 1", len(recs))
	}
	return recs[0]
}

// parseErrorsBody decodes the {"errors":{...}} JSON response body.
func parseErrorsBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse errors body %q: %v", rec.Body.String(), err)
	}
	return body.Errors
}

// TestSection01MissingNamesRejected verifies that a section 01 save with empty
// first_name and last_name returns 400 with both JSON errors and persists no
// record.
func TestSection01MissingNamesRejected(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"first_name": {""},
		"last_name":  {""},
		"event":      {fx.event},
		"id":         {""},
	}
	rec := doSection01Post(t, srv, admin, true, form)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	errs := parseErrorsBody(t, rec)
	if errs["first_name"] != "First name is required" {
		t.Errorf("first_name error = %q, want %q", errs["first_name"], "First name is required")
	}
	if errs["last_name"] != "Last name is required" {
		t.Errorf("last_name error = %q, want %q", errs["last_name"], "Last name is required")
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records = %d, want 0 (no record created)", n)
	}
}

// TestSection01MissingLastNameOnly verifies that an empty last_name alone
// returns 400 with only the last_name error and persists no record.
func TestSection01MissingLastNameOnly(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"first_name": {"Jane"},
		"last_name":  {""},
		"event":      {fx.event},
		"id":         {""},
	}
	rec := doSection01Post(t, srv, admin, true, form)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	errs := parseErrorsBody(t, rec)
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly one error", errs)
	}
	if errs["last_name"] != "Last name is required" {
		t.Errorf("last_name error = %q, want %q", errs["last_name"], "Last name is required")
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records = %d, want 0 (no record created)", n)
	}
}

// TestSection01SavesJoinedName verifies that populated first/last names save
// successfully as the joined full name in the single name column.
func TestSection01SavesJoinedName(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"first_name": {"Jane"},
		"last_name":  {"Doe"},
		"event":      {fx.event},
		"id":         {""},
	}
	rec := doSection01Post(t, srv, admin, true, form)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if loc := rec.Header().Get("HX-Redirect"); !strings.HasPrefix(loc, "/intake/") {
		t.Fatalf("HX-Redirect = %q, want prefix /intake/", loc)
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
}

// TestSection01LegacyNameSaves verifies the backward-compatible single `name`
// field (no first/last keys) still saves.
func TestSection01LegacyNameSaves(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"name":  {"Jane Doe"},
		"event": {fx.event},
		"id":    {""},
	}
	rec := doSection01Post(t, srv, admin, true, form)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
}

// TestSection01NoJSRedirect verifies the no-JS fallback: a valid save without
// the HX-Request header returns a 303 redirect to the resume URL.
func TestSection01NoJRedirect(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"first_name": {"Jane"},
		"last_name":  {"Doe"},
		"event":      {fx.event},
		"id":         {""},
	}
	rec := doSection01Post(t, srv, admin, false, form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	id := firstIntakeRecord(t, srv).Id
	if loc := rec.Header().Get("Location"); loc != "/intake/"+id {
		t.Fatalf("Location = %q, want %q", loc, "/intake/"+id)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
}

// TestSection02AutosaveSkipsNameValidation verifies that autosaving another
// section (02) on an existing record does not require the name fields and
// leaves the stored name untouched.
func TestSection02AutosaveSkipsNameValidation(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	// Create the record via a valid section 01 save.
	rec01 := doSection01Post(t, srv, admin, true, url.Values{
		"first_name": {"Jane"},
		"last_name":  {"Doe"},
		"event":      {fx.event},
		"id":         {""},
	})
	if rec01.Code != http.StatusAccepted {
		t.Fatalf("seed save status = %d, want 202", rec01.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	// Section 02 autosave with no name fields must succeed and keep the name.
	form := url.Values{
		"id":              {id},
		"homelessFactors": {"living in car"},
	}
	rec := doSectionPost(t, srv, admin, true, "02", form)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
	if got := firstIntakeRecord(t, srv).GetString("homelessFactors"); got != "living in car" {
		t.Fatalf("homelessFactors = %q, want %q", got, "living in car")
	}
}
