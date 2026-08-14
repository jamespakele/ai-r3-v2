package server

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"r3-intake/internal/assets"
)

// TestAdminEventsRender parses the embedded template and renders the admin
// page with populated events, guarding against template parse errors and
// missing-field errors on the new blocks.
func TestAdminEventsRender(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := &AdminView{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		Sites: []Site{
			{ID: "site1", Name: "Kona", Active: true},
			{ID: "site2", Name: "Waianae", Active: false},
		},
		Users: []UserRow{{ID: "u1", Name: "Case", Email: "cm@r3.local", Role: "case_manager"}},
		Events: []EventRow{
			{ID: "ev1", Name: "Summer Program", SiteName: "Kona", StartDate: "2026-08-01", EndDate: "2026-08-14", Enrolled: 3, Status: "active"},
			{ID: "ev2", Name: "Past Program", SiteName: "Waianae", StartDate: "2026-05-01", EndDate: "2026-05-30", Enrolled: 0, Status: "completed"},
		},
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "admin", view); err != nil {
		t.Fatalf("render admin: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Events", "Summer Program", "Past Program", "Kona", "Waianae",
		"2026-08-01 – 2026-08-14",
		`event-status-active">active`, `event-status-completed">completed`,
		`href="/attendance?event=ev1"`,
		`action="/admin/events"`, `Create Event`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("admin output missing %q", want)
		}
	}
	// Inactive site must NOT appear as a location option in the select.
	if strings.Contains(out, `value="site2"`) {
		t.Errorf("admin output unexpectedly includes inactive site option")
	}
	// Report button appears only for completed events.
	if !strings.Contains(out, `href="/admin/events/ev2/report"`) {
		t.Errorf("admin output missing Report link for completed event")
	}
	if strings.Contains(out, `href="/admin/events/ev1/report"`) {
		t.Errorf("admin output unexpectedly includes Report link for active event")
	}

	// Validation error path: preserves submitted values and shows the message.
	viewErr := &AdminView{
		UserName: "Admin", Role: "admin", IsAdmin: true,
		EventError: "Event name and location are required.",
		EventName:  "Kept Name", EventStart: "2026-08-01",
		Sites:  []Site{{ID: "site1", Name: "Kona", Active: true}},
		Events: []EventRow{},
	}
	var ebuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&ebuf, "admin", viewErr); err != nil {
		t.Fatalf("render admin (error): %v", err)
	}
	eout := ebuf.String()
	for _, want := range []string{"Event name and location are required.", `value="Kept Name"`, `value="2026-08-01"`} {
		if !strings.Contains(eout, want) {
			t.Errorf("admin error output missing %q", want)
		}
	}

	// The event-report placeholder renders.
	rview := &AdminView{
		UserName: "Admin", Role: "admin", IsAdmin: true,
		EventID: "ev2", EventName: "Past Program", EventStatus: "completed",
	}
	var rbuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&rbuf, "event-report", rview); err != nil {
		t.Fatalf("render event-report: %v", err)
	}
	rout := rbuf.String()
	for _, want := range []string{"Report — Past Program", "CSV export will be added in Epic 3."} {
		if !strings.Contains(rout, want) {
			t.Errorf("event-report output missing %q", want)
		}
	}
}

// findEventByName returns the first events record with the given name, or nil.
func findEventByName(t *testing.T, srv *Server, name string) *core.Record {
	t.Helper()
	col, err := srv.pb.FindCollectionByNameOrId("events")
	if err != nil {
		t.Fatalf("events collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id, "name='"+name+"'", "", 1, 0)
	if err != nil {
		t.Fatalf("find event: %v", err)
	}
	if len(recs) == 0 {
		return nil
	}
	return recs[0]
}

// doEventCreate POSTs the Create Event form to /admin/events (no trailing
// slash) with the given form values.
func doEventCreate(srv *Server, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/events", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestAdminEventCreateRouting proves a POST to /admin/events (the form action,
// no trailing slash) is handled directly by the explicit route and creates the
// event, instead of 301-redirecting to the /admin/events/ subtree (which is
// GET-only and 404s on the follow-up GET).
func TestAdminEventCreateRouting(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"name":        {"New Event"},
		"site":        {fx.site},
		"start_date":  {"2026-08-01"},
		"end_date":    {"2026-08-14"},
		"description": {"optional"},
	}
	rec := doEventCreate(srv, admin, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (event created)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=events" {
		t.Errorf("Location = %q, want /admin?tab=events", loc)
	}
	ev := findEventByName(t, srv, "New Event")
	if ev == nil {
		t.Fatal("expected event 'New Event' to be created, got none")
	}
	if got := ev.GetString("site"); got != fx.site {
		t.Errorf("event site = %q, want %q", got, fx.site)
	}
	if got := ev.GetString("status"); got != "active" {
		t.Errorf("event status = %q, want active", got)
	}
}

// TestAdminEventCreateValidation proves a POST with an empty name re-renders
// the admin page (200) with the validation error through the new route.
func TestAdminEventCreateValidation(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"name":       {""},
		"site":       {fx.site},
		"start_date": {"2026-08-01"},
		"end_date":   {"2026-08-14"},
	}
	rec := doEventCreate(srv, admin, form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (validation re-render)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Event name and location are required.") {
		t.Errorf("validation response missing error message")
	}
	if ev := findEventByName(t, srv, ""); ev != nil {
		t.Errorf("expected NO event created on validation failure, got one")
	}
}

// TestAdminEventCreateAuthBoundary proves an unauthenticated POST to
// /admin/events is rejected by requireRole and never reaches adminEventAdd,
// so no event is created.
func TestAdminEventCreateAuthBoundary(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)

	form := url.Values{
		"name":        {"AuthBoundary Event"},
		"site":        {fx.site},
		"start_date":  {"2026-08-01"},
		"end_date":    {"2026-08-14"},
		"description": {"should not be created"},
	}

	rec := doEventCreate(srv, nil, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 redirect to /login", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
	if ev := findEventByName(t, srv, "AuthBoundary Event"); ev != nil {
		t.Fatalf("expected no event created for unauthenticated request, got %q", ev.GetString("name"))
	}
}

// TestAdminEventCreateNonAdminRejected proves a case_manager POST to
// /admin/events is rejected by requireRole and never reaches adminEventAdd,
// so no event is created. The session id is fx.admin1 because requireRole only
// inspects the signed role, not the DB record.
func TestAdminEventCreateNonAdminRejected(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	cm := cmCookie(srv, fx.admin1)

	form := url.Values{
		"name":        {"NonAdmin Event"},
		"site":        {fx.site},
		"start_date":  {"2026-08-01"},
		"end_date":    {"2026-08-14"},
		"description": {"should not be created"},
	}

	rec := doEventCreate(srv, cm, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 redirect to /login", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
	if ev := findEventByName(t, srv, "NonAdmin Event"); ev != nil {
		t.Fatalf("expected no event created for non-admin request, got %q", ev.GetString("name"))
	}
}

// TestAdminEventCreateGetNoCreate proves a plain GET to /admin/events (no
// trailing slash) does not create an event. Go ServeMux auto-redirects
// /admin/events to the /admin/events/ subtree (301).
func TestAdminEventCreateGetNoCreate(t *testing.T) {
	srv := newTestServer(t)
	_ = seedToggleData(t, srv.pb)
	admin := adminCookie(srv, "admin-id-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/events", nil)
	req.AddCookie(admin)
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301 redirect to /admin/events/", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/events/" {
		t.Errorf("Location = %q, want /admin/events/", loc)
	}
	if ev := findEventByName(t, srv, "GET Event"); ev != nil {
		t.Fatalf("GET /admin/events must not create an event, got %q", ev.GetString("name"))
	}
}
