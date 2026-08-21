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
	"r3-intake/internal/assets"
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

// validSection01Form builds a fully valid section-01 form (the 10 fields the
// sec-01 form submits). The three sec-02 radios are deliberately absent: in
// the real DOM they live in the separate sec-02 form, so a sec-01 POST never
// carries them. Tests that need a complete record save sec-02 first (see
// seedCompleteRecord). overrides replace or delete keys by name.
func validSection01Form(fx sectionFixture, overrides map[string][]string) url.Values {
	v := url.Values{
		"event":          {fx.event},
		"first_name":     {"Jane"},
		"last_name":      {"Doe"},
		"dob":            {"08/21/1985"},
		"contact":        {"(808) 555-0100"},
		"race":           {"white"},
		"sexAtBirth":     {"female"},
		"servedMilitary": {"no"},
		"hasPets":        {"no"},
		"employment":     {"employed"},
		"id":             {""},
	}
	for k, vals := range overrides {
		if len(vals) == 0 {
			v.Del(k)
			continue
		}
		v.Del(k)
		for _, val := range vals {
			v.Add(k, val)
		}
	}
	return v
}

// validSection02Form builds the sec-02 radio fields the 12-field gate also
// requires, plus the given id.
func validSection02Form(id string) url.Values {
	return url.Values{
		"id":              {id},
		"mentalHealth":    {"no"},
		"substanceUse":    {"no"},
		"fleeingViolence": {"no"},
	}
}

// seedCompleteRecord mirrors the real save flow: the sec-02 form autosaves
// first (creating the record), then sec-01 saves with the returned id so
// validateRecord sees the persisted sec-02 values. Returns the record id.
func seedCompleteRecord(t *testing.T, srv *Server, cookie *http.Cookie, fx sectionFixture, hx bool) string {
	t.Helper()
	rec02 := doSectionPost(t, srv, cookie, hx, "02", validSection02Form(""))
	if rec02.Code != http.StatusAccepted {
		t.Fatalf("sec-02 seed status = %d, want 202", rec02.Code)
	}
	id := firstIntakeRecord(t, srv).Id
	rec01 := doSection01Post(t, srv, cookie, hx, validSection01Form(fx, map[string][]string{"id": {id}}))
	want := http.StatusNoContent
	if !hx {
		want = http.StatusSeeOther
	}
	if rec01.Code != want {
		t.Fatalf("sec-01 seed status = %d, want %d", rec01.Code, want)
	}
	return id
}

// TestSection01MissingEachFieldRejected verifies that a section-01 save
// missing any one of the 12 required fields returns 400 with the JSON error
// key for that field and persists no record. For the three sec-02 radio
// fields, "missing" means the record has no saved sec-02 values (a fresh
// record whose sec-02 never autosaved) — that is the server-consistent edge
// the plan documents: the sec-01 POST does not carry sec-02 fields, so a
// new record can only pass the gate once sec-02 has been saved.
func TestSection01MissingEachFieldRejected(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string][]string
		want      string
	}{
		{"event", map[string][]string{"event": {}}, "event"},
		{"name", map[string][]string{"first_name": {}, "last_name": {}}, "first_name"},
		{"dob", map[string][]string{"dob": {}}, "dob"},
		{"contact", map[string][]string{"contact": {"808"}}, "contact"},
		{"race", map[string][]string{"race": {}}, "race"},
		{"sexAtBirth", map[string][]string{"sexAtBirth": {}}, "sexAtBirth"},
		{"servedMilitary", map[string][]string{"servedMilitary": {}}, "servedMilitary"},
		{"hasPets", map[string][]string{"hasPets": {}}, "hasPets"},
		{"employment", map[string][]string{"employment": {}}, "employment"},
		{"mentalHealth", map[string][]string{"mentalHealth": {}}, "mentalHealth"},
		{"substanceUse", map[string][]string{"substanceUse": {}}, "substanceUse"},
		{"fleeingViolence", map[string][]string{"fleeingViolence": {}}, "fleeingViolence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			fx := seedActiveEvent(t, srv.pb)
			admin := adminCookie(srv, fx.admin)

			form := validSection01Form(fx, tc.overrides)
			rec := doSection01Post(t, srv, admin, true, form)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			errs := parseErrorsBody(t, rec)
			if _, ok := errs[tc.want]; !ok {
				t.Errorf("errors = %v, want key %q present", errs, tc.want)
			}
			if n := countIntakeRecords(t, srv); n != 0 {
				t.Fatalf("intake records = %d, want 0 (no record created)", n)
			}
		})
	}
}

// TestSection01MultipleMissingReturnsAllErrors verifies that a badly
// incomplete save (only the event set) returns 400 with every missing
// required-field error key and persists no record.
func TestSection01MultipleMissingReturnsAllErrors(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"event": {fx.event},
		"id":    {""},
	}
	rec := doSection01Post(t, srv, admin, true, form)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	errs := parseErrorsBody(t, rec)
	for _, want := range []string{
		"first_name", "last_name", "dob", "contact", "race",
		"sexAtBirth", "servedMilitary", "hasPets", "employment",
		"mentalHealth", "substanceUse", "fleeingViolence",
	} {
		if _, ok := errs[want]; !ok {
			t.Errorf("errors = %v, want key %q present", errs, want)
		}
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records = %d, want 0 (no record created)", n)
	}
}

// TestSection01AllFieldsPresentSucceeds verifies the full valid save: the
// sec-02 autosave creates the record and returns the HX-Redirect (202), then
// sec-01 saves the remaining 9 fields with the new id (204) and persists the
// joined name plus the sec-02 radio values.
func TestSection01AllFieldsPresentSucceeds(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	// Sec-02 autosave first: creates the record, returns the resume URL.
	rec02 := doSectionPost(t, srv, admin, true, "02", validSection02Form(""))
	if rec02.Code != http.StatusAccepted {
		t.Fatalf("sec-02 status = %d, want 202", rec02.Code)
	}
	if loc := rec02.Header().Get("HX-Redirect"); !strings.HasPrefix(loc, "/intake/") {
		t.Fatalf("HX-Redirect = %q, want prefix /intake/", loc)
	}
	id := firstIntakeRecord(t, srv).Id

	rec := doSection01Post(t, srv, admin, true, validSection01Form(fx, map[string][]string{"id": {id}}))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
	if got := saved.GetString("mentalHealth"); got != "no" {
		t.Fatalf("mentalHealth = %q, want %q", got, "no")
	}
	if got := saved.GetString("event"); got != fx.event {
		t.Fatalf("event = %q, want %q", got, fx.event)
	}
}

// TestSection01ValidationFailurePersistsNothing is the explicit no-persist
// check: a failing save leaves zero intake records.
func TestSection01ValidationFailurePersistsNothing(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	form := url.Values{
		"event": {fx.event},
		"id":    {""},
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records before = %d, want 0", n)
	}
	rec := doSection01Post(t, srv, admin, true, form)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records after = %d, want 0 (failed save must not persist)", n)
	}
}

// TestSection01NoJSRedirect verifies the no-JS fallback: both section saves
// without the HX-Request header return a 303 redirect to the resume URL and
// the name persists.
func TestSection01NoJSRedirect(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec02 := doSectionPost(t, srv, admin, false, "02", validSection02Form(""))
	if rec02.Code != http.StatusSeeOther {
		t.Fatalf("sec-02 status = %d, want 303", rec02.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	rec := doSection01Post(t, srv, admin, false, validSection01Form(fx, map[string][]string{"id": {id}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/intake/"+id {
		t.Fatalf("Location = %q, want %q", loc, "/intake/"+id)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
}

// TestSection02AutosaveSkipsValidation verifies that autosaving another
// section (02) on an existing record does not trigger the 12-field gate and
// leaves the stored name untouched.
func TestSection02AutosaveSkipsValidation(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	id := seedCompleteRecord(t, srv, admin, fx, true)

	// Section 02 autosave with no required fields must succeed (not gated).
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
	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
	if got := saved.GetString("homelessFactors"); got != "living in car" {
		t.Fatalf("homelessFactors = %q, want %q", got, "living in car")
	}
}

// TestEmbeddedTemplateIncludesValidationUI verifies the rebuilt embed carries
// the new validation UI: validateAll/setGroupErr/clearGroupErr, the group
// error containers, and the validateAll-gated Save buttons — and that the
// removed saveAll/per-section-save wiring is gone.
func TestEmbeddedTemplateIncludesValidationUI(t *testing.T) {
	tmpl, err := assets.TemplateString()
	if err != nil {
		t.Fatalf("TemplateString: %v", err)
	}
	for _, want := range []string{
		"R3F.validateAll",
		"R3F.setGroupErr",
		"R3F.clearGroupErr",
		"event-group",
		"dob-group",
		"contact-group",
		"race-group",
		"sexAtBirth-group",
		"servedMilitary-group",
		"hasPets-group",
		"employment-group",
		"mentalHealth-group",
		"substanceUse-group",
		"fleeingViolence-group",
		"if(R3F.validateAll())",
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("embedded template missing %q", want)
		}
	}
	if strings.Contains(tmpl, "saveAll") {
		t.Errorf("embedded template still contains saveAll")
	}
	if strings.Contains(tmpl, "section-save-btn") {
		t.Errorf("embedded template still contains section-save-btn")
	}
}
