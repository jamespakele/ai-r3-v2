package server

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"r3-intake/internal/assets"
)

// TestAdminEventsRender parses the embedded template and renders the admin
// page with populated events plus the event-manage placeholder, guarding
// against template parse errors and missing-field errors on the new blocks.
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
		`href="/admin/events/ev1/manage"`, `href="/attendance?event=ev1"`,
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

	// The event-manage page renders with roster + search + status controls.
	mview := &AdminView{
		UserName: "Admin", Role: "admin", IsAdmin: true,
		EventID: "ev1", EventName: "Summer Program", EventSiteName: "Kona",
		EventStatus: "active", EventStart: "2026-08-01", EventEnd: "2026-08-14",
		EnrolledCount: 2, EventEnrolled: 3,
		Enrolled: []EnrolledRow{
			{IntakeID: "i1", Name: "Keola Kanakaole", EnrolledDate: "2026-08-01", DaysAttended: 8, TotalDays: 11, Rate: 73, LastPresent: "2026-08-11"},
			{IntakeID: "i2", Name: "Mahealani Torres", EnrolledDate: "2026-08-01", DaysAttended: 2, TotalDays: 11, Rate: 18, LastPresent: "2026-07-30"},
		},
	}
	var mbuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&mbuf, "event-manage", mview); err != nil {
		t.Fatalf("render event-manage: %v", err)
	}
	mout := mbuf.String()
	for _, want := range []string{
		"Summer Program", "Kona", "2026-08-01 – 2026-08-14",
		`event-status-active">active`,
		`href="/attendance?event=ev1"`, "View Matrix", "Back to Admin",
		"Enrolled (2)",
		`hx-get="/admin/events/ev1/enroll-search"`, "Add participant",
		"Keola Kanakaole", "8 / 11", "73%", "2026-08-11",
		"Mahealani Torres", "2 / 11", "18%", "2026-07-30",
		`action="/admin/events/ev1/unenroll"`, "Remove",
		"Complete", "Cancel", "Mark this event as cancelled?",
	} {
		if !strings.Contains(mout, want) {
			t.Errorf("event-manage output missing %q", want)
		}
	}
	if !strings.Contains(mout, `class="status-badge rate-good">73%`) {
		t.Error("expected a good (>=50%) rate badge for 73%")
	}
	if !strings.Contains(mout, `class="status-badge rate-low">18%`) {
		t.Error("expected a low (<50%) rate badge for 18%")
	}

	// A terminal-state event renders read-only: no status controls.
	doneView := &AdminView{
		UserName: "Admin", Role: "admin", IsAdmin: true,
		EventID: "ev2", EventName: "Past Program",
		EventStatus: "completed", EventEnrolled: 0,
	}
	var dbuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&dbuf, "event-manage", doneView); err != nil {
		t.Fatalf("render event-manage (completed): %v", err)
	}
	dout := dbuf.String()
	for _, want := range []string{
		"is read-only", "Enrollment and status changes are disabled",
		`event-status-completed">completed`,
	} {
		if !strings.Contains(dout, want) {
			t.Errorf("event-manage (completed) output missing %q", want)
		}
	}
	for _, notWant := range []string{"Complete", "Cancel", "Mark this event as cancelled?"} {
		if strings.Contains(dout, notWant) {
			t.Errorf("event-manage (completed) output unexpectedly contains %q", notWant)
		}
	}

	// A transition error re-renders the manage page with the message.
	errView := &AdminView{
		UserName: "Admin", Role: "admin", IsAdmin: true,
		EventID: "ev2", EventName: "Past Program",
		EventStatus: "completed", EventEnrolled: 0,
		EventStatusError: "Invalid status transition: completed → active",
	}
	var xbuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&xbuf, "event-manage", errView); err != nil {
		t.Fatalf("render event-manage (error): %v", err)
	}
	if !strings.Contains(xbuf.String(), "Invalid status transition: completed → active") {
		t.Errorf("event-manage (error) output missing transition error message")
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

// TestEnrollSearchResultsRender renders the enroll-search-results fragment and
// verifies it lists results, marks already-enrolled hits, and targets the
// correct event for enrollment.
func TestEnrollSearchResultsRender(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}
	view := EnrollSearchView{
		EventID: "ev1",
		Results: []EnrollSearchResult{
			{ID: "i1", Name: "Keola Kanakaole", SiteName: "Kona", Already: true},
			{ID: "i2", Name: "Mahealani Torres", SiteName: "Kona", Already: false},
		},
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "enroll-search-results", view); err != nil {
		t.Fatalf("render enroll-search-results: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Keola Kanakaole", "Mahealani Torres",
		`action="/admin/events/ev1/enroll"`,
		`name="intake_id" value="i2"`, "+ Enroll", "Already enrolled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("enroll-search-results output missing %q", want)
		}
	}
}

// TestEnrollmentStatsCompute exercises the days/rate/last-present computation
// that backs the roster (pure date math; no DB required).
func TestEnrollmentStatsCompute(t *testing.T) {
	// Event not yet started: totalDays 0 and no panic.
	{
		st, _ := time.Parse("2006-01-02", "2026-08-01")
		en, _ := time.Parse("2006-01-02", "2026-08-14")
		cp, _ := time.Parse("2006-01-02", "2026-07-20") // today before start
		if got := daysInRange(st.Format("2006-01-02"), en.Format("2006-01-02"), cp.Format("2006-01-02")); got != 0 {
			t.Errorf("expected 0 totalDays for not-yet-started event, got %d", got)
		}
	}
	// Mid-event: totalDays = start..today inclusive.
	{
		st, _ := time.Parse("2006-01-02", "2026-08-01")
		en, _ := time.Parse("2006-01-02", "2026-08-14")
		cp, _ := time.Parse("2006-01-02", "2026-08-05")
		if got := daysInRange(st.Format("2006-01-02"), en.Format("2006-01-02"), cp.Format("2006-01-02")); got != 5 {
			t.Errorf("expected 5 totalDays for Aug 1-5, got %d", got)
		}
	}
	// Event ended: totalDays caps at end (start..end inclusive).
	{
		st, _ := time.Parse("2006-01-02", "2026-08-01")
		en, _ := time.Parse("2006-01-02", "2026-08-14")
		cp, _ := time.Parse("2006-01-02", "2026-09-01")
		if got := daysInRange(st.Format("2006-01-02"), en.Format("2006-01-02"), cp.Format("2006-01-02")); got != 14 {
			t.Errorf("expected 14 totalDays for full ended event, got %d", got)
		}
	}
	// Rate computation: 8 of 11 days => 72%.
	{
		if got := enrollmentRate(8, 11); got != 72 {
			t.Errorf("expected rate 72 for 8/11, got %d", got)
		}
	}
	// Division-by-zero guard: 0 totalDays => 0 rate, no panic.
	{
		if got := enrollmentRate(0, 0); got != 0 {
			t.Errorf("expected rate 0 for 0/0, got %d", got)
		}
	}
}

// TestEventStatusTransition covers the legal and illegal lifecycle transitions.
func TestEventStatusTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"active", "completed", true},
		{"active", "cancelled", true},
		{"active", "active", false},
		{"active", "", false},
		{"active", "deleted", false},
		{"completed", "active", false},
		{"completed", "cancelled", false},
		{"cancelled", "completed", false},
		{"cancelled", "active", false},
	}
	for _, tt := range tests {
		got := validEventTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("validEventTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
