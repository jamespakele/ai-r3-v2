package server

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

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

	// The event-manage placeholder renders.
	mview := &AdminView{UserName: "Admin", Role: "admin", IsAdmin: true, EventName: "Summer Program"}
	var mbuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&mbuf, "event-manage", mview); err != nil {
		t.Fatalf("render event-manage: %v", err)
	}
	mout := mbuf.String()
	for _, want := range []string{"Manage Enrollment — Summer Program", "Back to Admin"} {
		if !strings.Contains(mout, want) {
			t.Errorf("event-manage output missing %q", want)
		}
	}
}
