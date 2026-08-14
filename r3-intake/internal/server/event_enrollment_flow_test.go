package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// enrollDateRe matches the YYYY-MM-DD shape handleEventEnroll writes to
// enrolled_date. Tests assert this structural shape, never an exact date,
// because the value is HST "today"-relative.
var enrollDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// saveActiveEnrollment records one event_enrollment row with deleted=false.
// The shared saveEnrollment helper does NOT set deleted, and the roster filter
// (deleted=false) would exclude a NULL deleted value, so tests must set it
// explicitly. This stays valid after the sibling filter fix is merged.
func saveActiveEnrollment(t *testing.T, pb *pocketbase.PocketBase, event, intake string) {
	t.Helper()
	col, err := pb.FindCollectionByNameOrId("event_enrollment")
	if err != nil {
		t.Fatalf("event_enrollment collection: %v", err)
	}
	r := core.NewRecord(col)
	r.Set("event", event)
	r.Set("intake", intake)
	r.Set("deleted", false)
	if err := pb.Save(r); err != nil {
		t.Fatalf("save enrollment: %v", err)
	}
}

// doEnrollPost POSTs to the given path with CSRF and an optional cookie.
// hx=true sets HX-Request (fragment render); hx=false omits it (no-JS 303
// fallback).
func doEnrollPost(srv *Server, cookie *http.Cookie, path string, form url.Values, hx bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// doEnrollSearch GETs the enroll-search endpoint with CSRF and cookie.
func doEnrollSearch(srv *Server, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// countEnrollments returns the number of event_enrollment rows for an event,
// ignoring the deleted flag, so record counts are independent of the roster
// filter.
func countEnrollments(t *testing.T, srv *Server, eventID string) int {
	t.Helper()
	col, err := srv.eventEnrollmentCollection()
	if err != nil {
		t.Fatalf("event_enrollment collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id, fmt.Sprintf("event='%s'", eventID), "", 1000, 0)
	if err != nil {
		t.Fatalf("query enrollments: %v", err)
	}
	return len(recs)
}

// findEnrollment returns the event_enrollment row for (event, intake), or nil.
func findEnrollment(t *testing.T, srv *Server, eventID, intakeID string) *core.Record {
	t.Helper()
	col, err := srv.eventEnrollmentCollection()
	if err != nil {
		t.Fatalf("event_enrollment collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id,
		fmt.Sprintf("event='%s' && intake='%s'", eventID, intakeID), "", 1, 0)
	if err != nil {
		t.Fatalf("query enrollment: %v", err)
	}
	if len(recs) == 0 {
		return nil
	}
	return recs[0]
}

// TestEnrollFlowEndToEnd drives a single admin enroll POST through the real
// route and asserts both the rendered roster fragment and the resulting
// event_enrollment record.
func TestEnrollFlowEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	form := url.Values{"intake_id": {fx.iInSite1}}
	rec := doEnrollPost(srv, adminCookie(srv, "admin-test"), "/admin/events/"+fx.ev1+"/enroll", form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("roster fragment missing participant name Alice")
	}
	if !strings.Contains(body, "roster-table") {
		t.Errorf("roster fragment missing roster table")
	}

	if got := countEnrollments(t, srv, fx.ev1); got != 1 {
		t.Fatalf("enrollment count = %d, want 1", got)
	}
	er := findEnrollment(t, srv, fx.ev1, fx.iInSite1)
	if er == nil {
		t.Fatal("expected enrollment record for (ev1, iInSite1)")
	}
	if er.GetBool("deleted") {
		t.Errorf("enrollment deleted = true, want false")
	}
	if ed := er.GetString("enrolled_date"); !enrollDateRe.MatchString(ed) {
		t.Errorf("enrolled_date = %q, want YYYY-MM-DD", ed)
	}
}

// TestEnrollIdempotent proves that a second enroll POST for the same
// (event, intake) is a no-op: only one record exists and the roster lists the
// participant exactly once.
func TestEnrollIdempotent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	form := url.Values{"intake_id": {fx.iInSite1}}
	rec1 := doEnrollPost(srv, adminCookie(srv, "admin-test"), "/admin/events/"+fx.ev1+"/enroll", form, true)
	rec2 := doEnrollPost(srv, adminCookie(srv, "admin-test"), "/admin/events/"+fx.ev1+"/enroll", form, true)
	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("enroll statuses = %d, %d; want 200, 200", rec1.Code, rec2.Code)
	}
	if got := countEnrollments(t, srv, fx.ev1); got != 1 {
		t.Fatalf("enrollment count = %d, want 1", got)
	}
	// One roster row == one unenroll hx-post attribute in the fragment.
	wantRow := `hx-post="/admin/events/` + fx.ev1 + `/unenroll"`
	if got := strings.Count(rec2.Body.String(), wantRow); got != 1 {
		t.Errorf("roster lists %d rows, want exactly 1", got)
	}
}

// TestUnenrollSoftDeletes proves that unenroll keeps the record (soft-delete)
// but removes it from the active roster and flips deleted=true.
func TestUnenrollSoftDeletes(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)
	saveActiveEnrollment(t, srv.pb, fx.ev1, fx.iInSite1)

	form := url.Values{"intake_id": {fx.iInSite1}}
	rec := doEnrollPost(srv, adminCookie(srv, "admin-test"), "/admin/events/"+fx.ev1+"/unenroll", form, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No participants enrolled yet") {
		t.Errorf("roster does not show empty state after unenroll")
	}
	if got := countEnrollments(t, srv, fx.ev1); got != 1 {
		t.Fatalf("enrollment count = %d, want 1 (soft-delete preserves row)", got)
	}
	er := findEnrollment(t, srv, fx.ev1, fx.iInSite1)
	if er == nil {
		t.Fatal("expected enrollment record to still exist")
	}
	if !er.GetBool("deleted") {
		t.Errorf("enrollment deleted = false, want true")
	}
}

// TestEnrollSearch exercises the same-site participant search: matching
// results render with + Enroll, single-char queries return empty, and a no
// match renders the empty-state message. Cross-site intakes are excluded in
// this worktree's site-restriction clause, so only their absence is asserted.
func TestEnrollSearch(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)
	cookie := adminCookie(srv, "admin-test")

	search := func(q string) string {
		return doEnrollSearch(srv, cookie, "/admin/events/"+fx.ev1+"/enroll-search?name="+q).Body.String()
	}

	if b := search("Al"); !strings.Contains(b, "Alice") || !strings.Contains(b, "+ Enroll") {
		t.Errorf("search Al: want Alice + '+ Enroll', got %q", b)
	}
	if b := search("Bo"); !strings.Contains(b, "Bob") {
		t.Errorf("search Bo: want Bob, got %q", b)
	}
	if b := search("Ca"); strings.Contains(b, "Carol") {
		t.Errorf("search Ca: Carol is in site2, should be excluded, got %q", b)
	}

	rec := doEnrollSearch(srv, cookie, "/admin/events/"+fx.ev1+"/enroll-search?name=A")
	if rec.Code != http.StatusOK {
		t.Fatalf("single-char search status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "" {
		t.Errorf("single-char search body = %q, want empty", got)
	}

	if b := search("zz"); !strings.Contains(b, "No matching participants") {
		t.Errorf("search zz: want empty-state message, got %q", b)
	}
}

// TestEnrollSearchMarksAlreadyEnrolled proves that an already-enrolled intake
// renders a disabled "Already enrolled" button instead of "+ Enroll".
func TestEnrollSearchMarksAlreadyEnrolled(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)
	saveActiveEnrollment(t, srv.pb, fx.ev1, fx.iInSite1)

	b := doEnrollSearch(srv, adminCookie(srv, "admin-test"),
		"/admin/events/"+fx.ev1+"/enroll-search?name=Al").Body.String()
	if !strings.Contains(b, "Alice") {
		t.Errorf("search Al missing participant name, got %q", b)
	}
	if !strings.Contains(b, "Already enrolled") {
		t.Errorf("search Al missing disabled button, got %q", b)
	}
	if strings.Contains(b, "+ Enroll") {
		t.Errorf("search Al shows '+ Enroll' for an already-enrolled participant")
	}
}

// TestRosterRenderingWithStats seeds attendance and asserts the roster fragment
// renders the participant's attendance stats (days attended / total, rate
// badge, last present) computed the same way the handler does.
func TestRosterRenderingWithStats(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)
	saveActiveEnrollment(t, srv.pb, fx.ev1, fx.iInSite1)
	saveAttendance(t, srv.pb, fx.iInSite1, fx.site, fx.ev1, "2026-08-13", "present")
	saveAttendance(t, srv.pb, fx.iInSite1, fx.site, fx.ev1, "2026-08-14", "walk_in")

	req := httptest.NewRequest(http.MethodGet, "/admin/events/"+fx.ev1+"/manage", nil)
	addCSRFToRequest(req)
	req.AddCookie(adminCookie(srv, "admin-test"))
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("manage status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("roster missing participant Alice")
	}

	today := time.Now().In(hst).Format("2006-01-02")
	totalDays := daysInRange("2026-08-01", "2026-08-31", today)
	wantStat := fmt.Sprintf("2 / %d", totalDays)
	if !strings.Contains(body, wantStat) {
		t.Errorf("roster missing attendance stat %q", wantStat)
	}
	if !strings.Contains(body, "2026-08-14") {
		t.Errorf("roster missing last-present date 2026-08-14")
	}
	rate := enrollmentRate(2, totalDays)
	badge := "rate-low"
	if rate >= 50 {
		badge = "rate-good"
	}
	if !strings.Contains(body, badge) {
		t.Errorf("roster missing rate badge class %q", badge)
	}
	if !strings.Contains(body, fmt.Sprintf("%d%%", rate)) {
		t.Errorf("roster missing rate value %d%%", rate)
	}
}

// TestEnrollAuthBoundary proves the enroll, unenroll, and search routes are
// admin-only: case managers and anonymous users are redirected to /login and
// no enrollment record is created.
func TestEnrollAuthBoundary(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)
	form := url.Values{"intake_id": {fx.iInSite1}}

	cases := []struct {
		name   string
		cookie *http.Cookie
		method string
		path   string
	}{
		{"cm enroll", cmCookie(srv, fx.cm), http.MethodPost, "/admin/events/" + fx.ev1 + "/enroll"},
		{"cm unenroll", cmCookie(srv, fx.cm), http.MethodPost, "/admin/events/" + fx.ev1 + "/unenroll"},
		{"cm search", cmCookie(srv, fx.cm), http.MethodGet, "/admin/events/" + fx.ev1 + "/enroll-search?name=Al"},
		{"anon enroll", nil, http.MethodPost, "/admin/events/" + fx.ev1 + "/enroll"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			if tc.method == http.MethodGet {
				rec = doEnrollSearch(srv, tc.cookie, tc.path)
			} else {
				rec = doEnrollPost(srv, tc.cookie, tc.path, form, true)
			}
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != "/login" {
				t.Errorf("Location = %q, want /login", loc)
			}
		})
	}

	if got := countEnrollments(t, srv, fx.ev1); got != 0 {
		t.Errorf("enrollment count = %d, want 0 (case_manager enroll must not create a record)", got)
	}
}

// TestEnrollNoJSFallback proves that an enroll POST without the HTMX header
// gets a 303 redirect to the manage screen while still creating the record.
func TestEnrollNoJSFallback(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	form := url.Values{"intake_id": {fx.iInSite1}}
	rec := doEnrollPost(srv, adminCookie(srv, "admin-test"), "/admin/events/"+fx.ev1+"/enroll", form, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/events/"+fx.ev1+"/manage" {
		t.Errorf("Location = %q, want /admin/events/%s/manage", loc, fx.ev1)
	}
	if got := countEnrollments(t, srv, fx.ev1); got != 1 {
		t.Errorf("enrollment count = %d, want 1", got)
	}
}
