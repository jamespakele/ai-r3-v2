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
// carries them. Tests that need a complete record use seedCompleteRecord,
// which mirrors R3F.saveAll() by saving sec-02..sec-05 first, then sec-01
// last so the full 12-field gate passes. overrides replace or delete keys by
// name.
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

// validSection03Form builds a valid section-03 form.
func validSection03Form(id string) url.Values {
	return url.Values{
		"id":                  {id},
		"hmis":                {"on"},
		"hmisProvider":        {"Provider One"},
		"documents":           {"id"},
		"healthInsuranceDetail": {"insured"},
		"housing":             {"shelter"},
		"income":              {"ssi"},
		"casemanagerName":     {"Casey Manager"},
	}
}

// validSection04Form builds a valid section-04 form.
func validSection04Form(id string) url.Values {
	return url.Values{
		"id":        {id},
		"personal_0": {"answer 0"},
		"personal_1": {"answer 1"},
	}
}

// validSection05Form builds a valid section-05 form.
func validSection05Form(id string) url.Values {
	return url.Values{
		"id":           {id},
		"servicePlan_0": {"plan 0"},
		"servicePlan_1": {"plan 1"},
	}
}

// seedCompleteRecord mirrors R3F.saveAll(): sec-02 creates the record
// (202 + HX-Redirect for htmx, 303 without JS), sec-03..sec-05 save with the
// patched record id, and sec-01 runs last so the 12-field validateRecord
// gate sees the persisted sec-02 radio values and passes. Returns the id.
func seedCompleteRecord(t *testing.T, srv *Server, cookie *http.Cookie, fx sectionFixture, hx bool) string {
	t.Helper()

	firstWant := http.StatusAccepted
	if !hx {
		firstWant = http.StatusSeeOther
	}
	rec02 := doSectionPost(t, srv, cookie, hx, "02", validSection02Form(""))
	if rec02.Code != firstWant {
		t.Fatalf("sec-02 seed status = %d, want %d", rec02.Code, firstWant)
	}
	id := firstIntakeRecord(t, srv).Id

	subsequentWant := http.StatusNoContent
	if !hx {
		subsequentWant = http.StatusSeeOther
	}
	// sec-03/04/05 save with the patched record id so the seeded record
	// carries real section data.
	for _, sec := range []string{"03", "04", "05"} {
		var form url.Values
		switch sec {
		case "03":
			form = validSection03Form(id)
		case "04":
			form = validSection04Form(id)
		case "05":
			form = validSection05Form(id)
		}
		rec := doSectionPost(t, srv, cookie, hx, sec, form)
		if rec.Code != subsequentWant {
			t.Fatalf("sec-%s seed status = %d, want %d", sec, rec.Code, subsequentWant)
		}
	}

	// sec-01 LAST: full 12-field gate passes (sec-02 radios already persisted).
	rec01 := doSection01Post(t, srv, cookie, hx, validSection01Form(fx, map[string][]string{"id": {id}}))
	if rec01.Code != subsequentWant {
		t.Fatalf("sec-01 seed status = %d, want %d", rec01.Code, subsequentWant)
	}
	return id
}

// TestSection01MissingEachFieldRejected verifies that a section-01 save
// missing any one of the sec-01-own required fields returns 400 with the
// JSON error key for that field and persists no record. The sec-02-05 radio
// fields are not part of a sec-01 POST and are not checked by this gate.
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
	} {
		if _, ok := errs[want]; !ok {
			t.Errorf("errors = %v, want key %q present", errs, want)
		}
	}
	if n := countIntakeRecords(t, srv); n != 0 {
		t.Fatalf("intake records = %d, want 0 (no record created)", n)
	}
}

// TestSection01AllFieldsPresentSucceeds verifies the full valid save in the
// R3F.saveAll() ordering: sec-02 first creates the record and returns the
// HX-Redirect (202), then sec-01 saves the remaining 9 fields with the new id
// (204) and persists the joined name plus the sec-02 radio values.
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

// TestSection01FirstSaveSucceedsWithoutSec02 verifies the primary bug fix: a
// brand-new intake form's first POST /section/01 persists even though the
// sec-02-05 radio fields are empty (they are not part of the sec-01 form).
func TestSection01FirstSaveSucceedsWithoutSec02(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec := doSection01Post(t, srv, admin, true, validSection01Form(fx, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if loc := rec.Header().Get("HX-Redirect"); !strings.HasPrefix(loc, "/intake/") {
		t.Fatalf("HX-Redirect = %q, want prefix /intake/", loc)
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
	if got := saved.GetString("mentalHealth"); got != "" {
		t.Fatalf("mentalHealth = %q, want empty (sec-02 not submitted)", got)
	}
	if got := saved.GetString("substanceUse"); got != "" {
		t.Fatalf("substanceUse = %q, want empty (sec-02 not submitted)", got)
	}
	if got := saved.GetString("fleeingViolence"); got != "" {
		t.Fatalf("fleeingViolence = %q, want empty (sec-02 not submitted)", got)
	}
}

// TestSection01FirstSaveNoJSRedirect verifies the no-JS variant of the first
// save: without HX-Request, POST /section/01 still persists and redirects to
// the resume URL.
func TestSection01FirstSaveNoJSRedirect(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec := doSection01Post(t, srv, admin, false, validSection01Form(fx, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	saved := firstIntakeRecord(t, srv)
	if loc := rec.Header().Get("Location"); loc != "/intake/"+saved.Id {
		t.Fatalf("Location = %q, want %q", loc, "/intake/"+saved.Id)
	}
	if got := saved.GetString("name"); got != "Jane Doe" {
		t.Fatalf("name = %q, want %q", got, "Jane Doe")
	}
}

// TestSection01EditPreservesSec0205 verifies that re-saving sec-01 on an
// existing record does not clobber values written by sec-02. This is a
// regression guard for criterion 3.
func TestSection01EditPreservesSec0205(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	id := seedCompleteRecord(t, srv, admin, fx, true)

	form := validSection01Form(fx, map[string][]string{
		"id":         {id},
		"first_name": {"Janet"},
	})
	rec := doSection01Post(t, srv, admin, true, form)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("name"); got != "Janet Doe" {
		t.Fatalf("name = %q, want %q", got, "Janet Doe")
	}
	if got := saved.GetString("mentalHealth"); got != "no" {
		t.Fatalf("mentalHealth = %q, want %q", got, "no")
	}
	if got := saved.GetString("substanceUse"); got != "no" {
		t.Fatalf("substanceUse = %q, want %q", got, "no")
	}
	if got := saved.GetString("fleeingViolence"); got != "no" {
		t.Fatalf("fleeingViolence = %q, want %q", got, "no")
	}
}

// TestFinishFullGateBlocksIncomplete verifies the defense-in-depth complete-
// record gate still runs at /intake/{id}/finish. A first-save sec-01-only
// record has empty sec-02-05 values; requesting finish renders the page with
// errors for the three missing fields.
func TestFinishFullGateBlocksIncomplete(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec01 := doSection01Post(t, srv, admin, true, validSection01Form(fx, nil))
	if rec01.Code != http.StatusAccepted {
		t.Fatalf("sec-01 status = %d, want 202", rec01.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	req := httptest.NewRequest(http.MethodPost, "/intake/"+id+"/finish", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("finish status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, marker := range []string{
		`id="mentalHealth-error">Please select one.`,
		`id="substanceUse-error">Please select one.`,
		`id="fleeingViolence-error">Please select one.`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("finish page missing error marker %q", marker)
		}
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

// TestSaveAllServerFlow verifies the reordered saveAll sequence: sec-02 is
// posted first on a new record (creating it and returning HX-Redirect), the
// returned id is patched into the remaining forms, then sec-03/04/05/01
// return 204 and all sections persist against a single record.
func TestSaveAllServerFlow(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	sec01 := validSection01Form(fx, map[string][]string{"id": {""}})
	sec02 := validSection02Form("")
	sec03 := validSection03Form("")
	sec04 := validSection04Form("")
	sec05 := validSection05Form("")

	// Step 1: sec-02 creates the record and returns HX-Redirect.
	rec02 := doSectionPost(t, srv, admin, true, "02", sec02)
	if rec02.Code != http.StatusAccepted {
		t.Fatalf("sec-02 status = %d, want 202", rec02.Code)
	}
	loc := rec02.Header().Get("HX-Redirect")
	if loc == "" {
		t.Fatalf("sec-02 response missing HX-Redirect")
	}

	// Simulate patchRecordId by extracting the id from /intake/<id>.
	var id string
	if i := strings.LastIndex(loc, "/"); i >= 0 {
		id = loc[i+1:]
	} else {
		t.Fatalf("HX-Redirect %q does not contain /intake/<id>", loc)
	}

	sec01.Set("id", id)
	sec03.Set("id", id)
	sec04.Set("id", id)
	sec05.Set("id", id)

	// Steps 2-5: remaining sections return 204.
	for _, tc := range []struct {
		section string
		form    url.Values
	}{
		{"03", sec03},
		{"04", sec04},
		{"05", sec05},
		{"01", sec01},
	} {
		rec := doSectionPost(t, srv, admin, true, tc.section, tc.form)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("sec-%s status = %d, want 204", tc.section, rec.Code)
		}
	}
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1", n)
	}
	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("name"); got != "Jane Doe" {
		t.Errorf("name = %q, want %q", got, "Jane Doe")
	}
	if got := saved.GetString("mentalHealth"); got != "no" {
		t.Errorf("mentalHealth = %q, want %q", got, "no")
	}
	if got := saved.GetString("hmisProvider"); got != "Provider One" {
		t.Errorf("hmisProvider = %q, want %q", got, "Provider One")
	}
	if got := saved.GetStringSlice("personal"); len(got) == 0 || got[0] != "answer 0" {
		t.Errorf("personal = %v, want non-empty starting with answer 0", got)
	}
	if got := saved.GetStringSlice("servicePlan"); len(got) == 0 || got[0] != "plan 0" {
		t.Errorf("servicePlan = %v, want non-empty starting with plan 0", got)
	}
}

// TestSaveAllSurfaces400OnInvalidSection01 verifies the 400 path within the
// saveAll flow: sec-02 creates the record, sec-03/04/05 save, then sec-01
// fails the 12-field gate. The server surfaces a 400 JSON body (which
// R3F.applyErrors would consume) and persists nothing new for sec-01 — the
// record already exists from sec-02, so the correct assertion is count==1
// with an empty name, not count==0 (that would be the standalone-sec-01 case).
func TestSaveAllSurfaces400OnInvalidSection01(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	// sec-02 first: creates the record (202).
	rec02 := doSectionPost(t, srv, admin, true, "02", validSection02Form(""))
	if rec02.Code != http.StatusAccepted {
		t.Fatalf("sec-02 status = %d, want 202", rec02.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	for _, sec := range []string{"03", "04", "05"} {
		r := doSectionPost(t, srv, admin, true, sec, url.Values{"id": {id}})
		if r.Code != http.StatusNoContent {
			t.Fatalf("sec-%s status = %d, want 204", sec, r.Code)
		}
	}

	// sec-01 last but missing first_name+last_name (and the rest): server
	// surfaces a 400 JSON body R3F.applyErrors would consume. The record
	// already exists from sec-02; sec-01's failed gate persists nothing new.
	form := validSection01Form(fx, map[string][]string{
		"id":         {id},
		"first_name": {}, // drop
		"last_name":  {}, // drop
	})
	rec01 := doSection01Post(t, srv, admin, true, form)
	if rec01.Code != http.StatusBadRequest {
		t.Fatalf("sec-01 status = %d, want 400", rec01.Code)
	}
	if ct := rec01.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	errs := parseErrorsBody(t, rec01)
	if _, ok := errs["first_name"]; !ok {
		t.Errorf("errors = %v, want key %q present", errs, "first_name")
	}
	// Correct saveAll-flow assertion: record already created by sec-02;
	// failed sec-01 must not duplicate it nor persist the name.
	if n := countIntakeRecords(t, srv); n != 1 {
		t.Fatalf("intake records = %d, want 1 (sec-02 record persists; failed sec-01 must not create a duplicate)", n)
	}
	if got := firstIntakeRecord(t, srv).GetString("name"); got != "" {
		t.Fatalf("name = %q, want empty (failed sec-01 must not persist)", got)
	}
}

// TestEmbeddedTemplateIncludesValidationUI verifies the rebuilt embed carries
// the validation UI: validateAll/setGroupErr/clearGroupErr, the group
// error containers, the validateAll-gated Save buttons, and the restored
// R3F.saveAll/R3F.patchRecordId/R3F.applyErrors wiring.
func TestEmbeddedTemplateIncludesValidationUI(t *testing.T) {
	tmpl, err := assets.TemplateString()
	if err != nil {
		t.Fatalf("TemplateString: %v", err)
	}
	for _, want := range []string{
		"R3F.validateAll",
		"R3F.setGroupErr",
		"R3F.clearGroupErr",
		"R3F.saveAll",
		"R3F.patchRecordId",
		"R3F.applyErrors",
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
	// saveAll is restored and wired to both Save buttons.
	if !strings.Contains(tmpl, "R3F.saveAll") {
		t.Errorf("embedded template missing R3F.saveAll")
	}
	if !strings.Contains(tmpl, "R3F.patchRecordId") {
		t.Errorf("embedded template missing R3F.patchRecordId")
	}
	if !strings.Contains(tmpl, "R3F.applyErrors") {
		t.Errorf("embedded template missing R3F.applyErrors")
	}
	if c := strings.Count(tmpl, `if(R3F.validateAll()){R3F.saveAll()}`); c != 2 {
		t.Errorf("Save buttons wired to R3F.saveAll() = %d, want 2", c)
	}
	// Stale sec-01-only submit wiring must be gone from Save buttons.
	if strings.Contains(tmpl, `htmx.trigger(document.getElementById('sec-01'),'submit')`) {
		t.Errorf("embedded template still has stale htmx.trigger Save wiring")
	}
	// The old per-section section-save-btn class is still gone (kept assertion).
	if strings.Contains(tmpl, "section-save-btn") {
		t.Errorf("embedded template still contains section-save-btn")
	}
	if strings.Contains(tmpl, "htmx.trigger(document.getElementById('sec-01'),'submit')") {
		t.Errorf("stale htmx.trigger Save wiring still present")
	}
}
