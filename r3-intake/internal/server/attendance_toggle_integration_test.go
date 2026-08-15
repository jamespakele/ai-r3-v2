package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// toggleFixtures holds the record ids created by seedToggleData so tests can
// reference them.
type toggleFixtures struct {
	site, ev, admin1, iNoSite, iLocated string
}

// seedToggleData creates one active site, one event, one admin user, one
// intake with no assigned site, and one intake with the site assigned.
func seedToggleData(t *testing.T, pb *pocketbase.PocketBase) toggleFixtures {
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

	ev := save("ev", func() *core.Record {
		r := rec("events")
		r.Set("site", site)
		r.Set("name", "Morning Program")
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

	iNoSite := save("iNoSite", func() *core.Record {
		r := rec("intake")
		r.Set("name", "NoSite Bob")
		return r
	}())
	iLocated := save("iLocated", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Located Alice")
		r.Set("site", site)
		return r
	}())

	return toggleFixtures{site, ev, admin1, iNoSite, iLocated}
}

// doToggle POSTs the attendance toggle with the HTMX request header and the
// given form values.
func doToggle(srv *Server, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/attendance/toggle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestToggleNoLocation proves a participant with no assigned site cannot have
// their attendance dot toggled — attendance requires a location.
func TestToggleNoLocation(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iNoSite},
		"date":      {"2026-08-13"},
		"site_id":   {""},
		"event_id":  {fx.ev},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}
	rec := doToggle(srv, admin, form)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (attendance requires a location)", rec.Code)
	}

	att := findAttendance(t, srv, fx.iNoSite, "2026-08-13")
	if att != nil {
		t.Fatalf("expected NO attendance record for no-location intake, got one")
	}
}

// TestToggleLocated proves a located participant's toggle stores the event
// and no longer writes a redundant site on the attendance row (location is
// derived from the event).
func TestToggleLocated(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-13"},
		"site_id":   {fx.site},
		"event_id":  {fx.ev},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}
	rec := doToggle(srv, admin, form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	att := findAttendance(t, srv, fx.iLocated, "2026-08-13")
	if att == nil {
		t.Fatalf("expected attendance record for located intake")
	}
	if att.GetString("site") != "" {
		t.Errorf("site = %q, want empty (location derived from event)", att.GetString("site"))
	}
	if att.GetString("event") != fx.ev {
		t.Errorf("event = %q, want %q", att.GetString("event"), fx.ev)
	}
}

// TestToggleRequiresEvent proves a toggle without an event_id is rejected
// with a 400 and writes no attendance record.
func TestToggleRequiresEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-13"},
		"site_id":   {fx.site},
		"event_id":  {""},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}
	rec := doToggle(srv, admin, form)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (event required)", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "an event must be selected before recording attendance" {
		t.Errorf("body = %q, want canonical message", body)
	}
	if att := findAttendance(t, srv, fx.iLocated, "2026-08-13"); att != nil {
		t.Fatalf("expected NO attendance record without an event, got one")
	}
}

// TestToggleStoresEvent proves a toggle with an event_id stores that event on
// the created record.
func TestToggleStoresEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-13"},
		"site_id":   {fx.site},
		"event_id":  {fx.ev},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}
	rec := doToggle(srv, admin, form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	att := findAttendance(t, srv, fx.iLocated, "2026-08-13")
	if att == nil {
		t.Fatalf("expected attendance record")
	}
	if att.GetString("event") != fx.ev {
		t.Errorf("event = %q, want %q", att.GetString("event"), fx.ev)
	}
}

// TestWalkinRequiresEvent proves a walk-in without an event_id is rejected
// with a 400 and writes no attendance record.
func TestWalkinRequiresEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iLocated},
		"site_id":   {fx.site},
	}
	req := httptest.NewRequest(http.MethodPost, "/attendance/walkin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (event required)", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "an event must be selected before recording attendance" {
		t.Errorf("body = %q, want canonical message", body)
	}
	today := time.Now().In(hst).Format("2006-01-02")
	if att := findAttendance(t, srv, fx.iLocated, today); att != nil {
		t.Fatalf("expected NO walk-in record without an event, got one")
	}
}

// TestToggleEventScoped proves uniqueness is keyed on (event, intake, date):
// toggling the same full key twice updates the existing record (no duplicate),
// while a different event for the same (intake, date) creates a separate
// record.
func TestToggleEventScoped(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	// Create a second event on the same site.
	ev2 := func() string {
		col, err := srv.pb.FindCollectionByNameOrId("events")
		if err != nil {
			t.Fatalf("events collection: %v", err)
		}
		r := core.NewRecord(col)
		r.Set("site", fx.site)
		r.Set("name", "Evening Session")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		if err := srv.pb.Save(r); err != nil {
			t.Fatalf("save ev2: %v", err)
		}
		return r.Id
	}()
	ev2ID := ev2

	base := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-13"},
		"site_id":   {fx.site},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}

	// Toggle ev on → single record.
	f1 := cloneValues(base)
	f1.Set("event_id", fx.ev)
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev on = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 1 {
		t.Errorf("after toggling ev on, record count = %d, want 1", n)
	}
	// Toggle ev off → record deleted (toggle semantics), so the same full key
	// never duplicates.
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev off = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 0 {
		t.Errorf("after toggling ev off, record count = %d, want 0", n)
	}
	// Toggle ev on again.
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev on #2 = %d, want 200", rec.Code)
	}
	// Toggle ev2 (different event, same intake/date) → coexists with ev's
	// record: two distinct records keyed by different events.
	f2 := cloneValues(base)
	f2.Set("event_id", ev2ID)
	if rec := doToggle(srv, admin, f2); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev2 = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 2 {
		t.Errorf("record count with two events = %d, want 2", n)
	}
}

// TestToggleScopesPerEvent proves the idempotency filter keys on
// (event, intake, date): toggling a second event creates a separate record
// rather than clobbering the first event's record.
func TestToggleScopesPerEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	// A second event for the same site.
	ev2 := func() string {
		col, err := srv.pb.FindCollectionByNameOrId("events")
		if err != nil {
			t.Fatalf("events collection: %v", err)
		}
		r := core.NewRecord(col)
		r.Set("site", fx.site)
		r.Set("name", "Evening Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		if err := srv.pb.Save(r); err != nil {
			t.Fatalf("save event2: %v", err)
		}
		return r.Id
	}()
	ev2ID := ev2

	base := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-13"},
		"site_id":   {fx.site},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
	}

	// Toggle ev on → single record.
	f1 := cloneValues(base)
	f1.Set("event_id", fx.ev)
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev on = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 1 {
		t.Errorf("after toggling ev on, record count = %d, want 1", n)
	}
	// Toggle ev off → record deleted (toggle semantics), so the same full key
	// never duplicates.
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev off = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 0 {
		t.Errorf("after toggling ev off, record count = %d, want 0", n)
	}
	// Toggle ev on again.
	if rec := doToggle(srv, admin, f1); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev on #2 = %d, want 200", rec.Code)
	}
	// Toggle ev2 (different event, same intake/date) → coexists with ev's
	// record: two distinct records keyed by different events.
	f2 := cloneValues(base)
	f2.Set("event_id", ev2ID)
	if rec := doToggle(srv, admin, f2); rec.Code != http.StatusOK {
		t.Fatalf("toggle ev2 = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.iLocated, "2026-08-13"); n != 2 {
		t.Errorf("record count with two events = %d, want 2", n)
	}

	// Two distinct records coexist: one per event, same intake/date.
	col, err := srv.pb.FindCollectionByNameOrId("attendance")
	if err != nil {
		t.Fatalf("attendance collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id,
		"intake='"+fx.iLocated+"' && date='2026-08-13'", "", 10, 0)
	if err != nil {
		t.Fatalf("find attendance: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("record count = %d, want 2 (one per event)", len(recs))
	}
	events := map[string]bool{}
	for _, r := range recs {
		events[r.GetString("event")] = true
	}
	if !events[fx.ev] || !events[ev2ID] {
		t.Errorf("expected records for both events, got %v", events)
	}
}

// cloneValues returns a copy of url.Values.
func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vals := range v {
		for _, val := range vals {
			out.Add(k, val)
		}
	}
	return out
}

// doWalkin POSTs the walk-in form to /attendance/walkin with CSRF and the
// given form values. Unlike doToggle it does not set the HTMX request header.
func doWalkin(srv *Server, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/attendance/walkin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestWalkinStoresNoSite proves the walk-in write path stores no site on the
// attendance row (location is derived from the event) and that the export
// resolves the location from the event's site.
func TestWalkinStoresNoSite(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	today := time.Now().In(hst).Format("2006-01-02")

	form := url.Values{
		"site_id":   {fx.site},
		"event_id":  {fx.ev},
		"intake_id": {fx.iLocated},
	}
	rec := doWalkin(srv, admin, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}

	att := findAttendance(t, srv, fx.iLocated, today)
	if att == nil {
		t.Fatalf("expected walk-in attendance record")
	}
	if att.GetString("site") != "" {
		t.Errorf("site = %q, want empty (location derived from event)", att.GetString("site"))
	}
	if att.GetString("event") != fx.ev {
		t.Errorf("event = %q, want %q", att.GetString("event"), fx.ev)
	}
	if att.GetString("status") != "walk_in" {
		t.Errorf("status = %q, want walk_in", att.GetString("status"))
	}

	rows, err := srv.loadExportRows("", fx.ev, today, today)
	if err != nil {
		t.Fatalf("loadExportRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("export rows = %d, want 1", len(rows))
	}
	if rows[0].SiteName != "Kona" {
		t.Errorf("SiteName = %q, want Kona (resolved from event's site)", rows[0].SiteName)
	}
}
