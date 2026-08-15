package server

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"r3-intake/internal/assets"
)

func TestComputeSummary(t *testing.T) {
	tests := []struct {
		name string
		rows []MatrixRow
		days int
		want MatrixSummary
	}{
		{
			name: "empty rows",
			rows: nil,
			days: 0,
			want: MatrixSummary{},
		},
		{
			name: "present and walk-in counted in check-ins",
			rows: []MatrixRow{
				{PresentCount: 2, WalkInCount: 1, TotalDays: 5},
			},
			days: 5,
			want: MatrixSummary{TotalCheckIns: 3, ActiveParticipants: 1, Stopped: 0, AvgRate: 40},
		},
		{
			name: "dropout and average rounds down",
			rows: []MatrixRow{
				{PresentCount: 1},
				{PresentCount: 0, IsDropout: true},
			},
			days: 4,
			want: MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 1, Stopped: 1, AvgRate: 12},
		},
		{
			name: "walk-in is not an active participant",
			rows: []MatrixRow{
				{WalkInCount: 1, PresentCount: 0},
			},
			days: 3,
			want: MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 0, Stopped: 0, AvgRate: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSummary(tt.rows, tt.days)
			if got != tt.want {
				t.Errorf("computeSummary(%v, %d) = %+v, want %+v", tt.rows, tt.days, got, tt.want)
			}
		})
	}
}

// TestMatrixContentRender parses the embedded template and renders the
// matrix-content and stat-cards partials with a populated view, guarding
// against template parse errors and missing-field errors on the new blocks.
func TestMatrixContentRender(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := MatrixViewData{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		SiteID:   "site1",
		SiteName: "Kona",
		DateFrom: "2026-08-01",
		DateTo:   "2026-08-14",
		Dates:    []string{"2026-08-01", "2026-08-02"},
		Rows: []MatrixRow{
			{IntakeID: "i2", Name: "Bob", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i2", Date: "2026-08-01", Status: "absent", SiteID: "", From: "2026-08-01", To: "2026-08-14", Disabled: true},
					{IntakeID: "i2", Date: "2026-08-02", Status: "", SiteID: "", From: "2026-08-01", To: "2026-08-14", Disabled: true},
				}},
			{IntakeID: "i1", Name: "Alice", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i1", Date: "2026-08-01", Status: "present", SiteID: "site1", From: "2026-08-01", To: "2026-08-14"},
					{IntakeID: "i1", Date: "2026-08-02", Status: "walk_in", SiteID: "site1", From: "2026-08-01", To: "2026-08-14"},
				},
				PresentCount: 1, WalkInCount: 1},
		},
		Sites:   []Site{{ID: "site1", Name: "Kona"}},
		Events:  []Event{{ID: "ev1", Name: "Morning Program"}},
		Summary: MatrixSummary{TotalCheckIns: 2, ActiveParticipants: 1, Stopped: 0, AvgRate: 25},
		EventID: "ev1",
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "matrix-content", view); err != nil {
		t.Fatalf("render matrix-content: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Total check-ins", "Active participants", "Stopped", "Avg attendance rate",
		"Morning Program", "Alice",
		`>2</div><div class="stat-label">Total check-ins`, "25%",
		"dot-disabled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("matrix-content output missing %q", want)
		}
	}

	// The full-page matrix template must also render (no-JS path).
	var full bytes.Buffer
	if err := tpl.ExecuteTemplate(&full, "matrix", view); err != nil {
		t.Fatalf("render matrix: %v", err)
	}
	if !strings.Contains(full.String(), "Attendance") {
		t.Errorf("matrix output missing page title")
	}
}

// TestMatrixContentRenderEventRequired verifies that when no events exist,
// the matrix renders the create-an-event empty state, hides the walk-in
// panel, and disables every cell.
func TestMatrixContentRenderEventRequired(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := MatrixViewData{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		SiteID:   "site1",
		SiteName: "Kona",
		DateFrom: "2026-08-01",
		DateTo:   "2026-08-14",
		Dates:    []string{"2026-08-01", "2026-08-02"},
		Rows: []MatrixRow{
			{IntakeID: "i1", Name: "Alice", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i1", Date: "2026-08-01", Status: "present", SiteID: "site1", From: "2026-08-01", To: "2026-08-14", Disabled: true},
					{IntakeID: "i1", Date: "2026-08-02", Status: "", SiteID: "site1", From: "2026-08-01", To: "2026-08-14", Disabled: true},
				},
				PresentCount: 1},
		},
		Sites:         []Site{{ID: "site1", Name: "Kona"}},
		Events:        []Event{},
		EventID:       "",
		EventRequired: true,
		NoEvents:      true,
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "matrix-content", view); err != nil {
		t.Fatalf("render matrix-content: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Create an Event to track attendance.",
		"/admin",
		"Alice",
		"dot-disabled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("matrix-content (event required) output missing %q", want)
		}
	}
	for _, forbid := range []string{
		"Add walk-in",
		"walkin-search",
	} {
		if strings.Contains(out, forbid) {
			t.Errorf("matrix-content (event required) output should not contain %q", forbid)
		}
	}
}

// TestMatrixContentRenderDefaultsToFirstEvent verifies that when an event is
// selected (the server defaults to the first active event), the matrix
// renders that event as the selected option and no placeholder option.
func TestMatrixContentRenderDefaultsToFirstEvent(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := MatrixViewData{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		SiteID:   "site1",
		SiteName: "Kona",
		DateFrom: "2026-08-01",
		DateTo:   "2026-08-14",
		Dates:    []string{"2026-08-01", "2026-08-02"},
		Rows: []MatrixRow{
			{IntakeID: "i1", Name: "Alice", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i1", Date: "2026-08-01", Status: "present", SiteID: "site1", From: "2026-08-01", To: "2026-08-14"},
					{IntakeID: "i1", Date: "2026-08-02", Status: "", SiteID: "site1", From: "2026-08-01", To: "2026-08-14"},
				},
				PresentCount: 1},
		},
		Sites:   []Site{{ID: "site1", Name: "Kona"}},
		Events:  []Event{{ID: "ev1", Name: "Morning Program"}},
		Summary: MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 1, Stopped: 0, AvgRate: 50},
		EventID: "ev1",
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "matrix-content", view); err != nil {
		t.Fatalf("render matrix-content: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`<option value="ev1" selected>Morning Program</option>`,
		"Alice",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("matrix-content (defaulted event) output missing %q", want)
		}
	}
	for _, forbid := range []string{
		"Select an event…",
		"Create an Event to track attendance.",
	} {
		if strings.Contains(out, forbid) {
			t.Errorf("matrix-content (defaulted event) output should not contain %q", forbid)
		}
	}
}

// TestMatrixContentRenderEventScopedFormDates verifies that the walk-in
// panel's hidden from/to inputs and the matrix-cell toggle forms' hidden
// from/to inputs carry the event-scoped date range from the view model.
func TestMatrixContentRenderEventScopedFormDates(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := MatrixViewData{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		SiteID:   "site1",
		SiteName: "Kona",
		DateFrom: "2026-08-01", // event-scoped start
		DateTo:   "2026-08-31", // event-scoped end
		Dates:    []string{"2026-08-01", "2026-08-31"},
		Rows: []MatrixRow{
			{IntakeID: "i1", Name: "Alice", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i1", Date: "2026-08-01", Status: "present",
						SiteID: "site1", From: "2026-08-01", To: "2026-08-31"},
					{IntakeID: "i1", Date: "2026-08-31", Status: "absent",
						SiteID: "site1", From: "2026-08-01", To: "2026-08-31",
						Disabled: true},
				},
				PresentCount: 1, WalkInCount: 0},
		},
		Sites:   []Site{{ID: "site1", Name: "Kona"}},
		Events:  []Event{{ID: "ev1", Name: "Morning Program"}},
		Summary: MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 1, Stopped: 0, AvgRate: 50},
		EventID: "ev1",
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "matrix-content", view); err != nil {
		t.Fatalf("render matrix-content: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<input type="hidden" name="from" value="2026-08-01">`,
		`<input type="hidden" name="to" value="2026-08-31">`,
		`name="from" value="2026-08-01"`,
		`name="to" value="2026-08-31"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("matrix-content output missing event-scoped form date %q", want)
		}
	}
}

// TestExportCSVRecords verifies the pure CSV builder: header row, per-record
// rows with title-cased status and empty-relation cells, and a trailing
// summary row with the expected counts and rate.
func TestExportCSVRecords(t *testing.T) {
	rows := []ExportRow{
		{ParticipantName: "Alice", SiteName: "Hilo", EventName: "", Date: "2026-08-01", Status: "present", RecordedByName: "", CheckInTime: "", Note: "on time"},
		{ParticipantName: "Alice", SiteName: "Hilo", EventName: "Job Fair", Date: "2026-08-02", Status: "walk_in", RecordedByName: "Bob", CheckInTime: "", Note: "late, note with, comma"},
		{ParticipantName: "Bob", SiteName: "Hilo", EventName: "", Date: "2026-08-01", Status: "absent", RecordedByName: "", CheckInTime: "", Note: ""},
	}

	records := exportCSVRecords(rows)

	// Header row.
	wantHeader := []string{"Participant", "Site", "Event", "Date", "Status", "Recorded By", "Check-in Time", "Note"}
	for i, want := range wantHeader {
		if records[0][i] != want {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], want)
		}
	}

	// Data rows: status title-cased, empty relations stay empty (not "<nil>").
	if got := records[1][4]; got != "Present" {
		t.Errorf("row1 status = %q, want Present", got)
	}
	if got := records[1][2]; got != "" {
		t.Errorf("row1 event = %q, want empty", got)
	}
	if got := records[2][4]; got != "Walk-in" {
		t.Errorf("row2 status = %q, want Walk-in", got)
	}
	if got := records[2][7]; got != "late, note with, comma" {
		t.Errorf("row2 note = %q, want comma note", got)
	}
	if got := records[2][5]; got != "Bob" {
		t.Errorf("row2 recorded_by = %q, want Bob", got)
	}
	if got := records[3][4]; got != "Absent" {
		t.Errorf("row3 status = %q, want Absent", got)
	}
	if got := records[3][7]; got != "" {
		t.Errorf("row3 note = %q, want empty", got)
	}
	if got := records[3][5]; got != "" {
		t.Errorf("row3 recorded_by = %q, want empty", got)
	}

	// Summary row: 2 check-ins (present + walk_in), 2 unique participants,
	// 1 present over 2 unique x 2 days = 25% avg rate.
	sum := records[len(records)-1]
	wantSum := "Summary: 2 check-ins, 2 unique participants, 25% avg rate"
	if sum[0] != wantSum {
		t.Errorf("summary first cell = %q, want %q", sum[0], wantSum)
	}
	for i := 1; i < len(sum); i++ {
		if sum[i] != "" {
			t.Errorf("summary col %d = %q, want empty", i, sum[i])
		}
	}
}

// TestRequireEventID verifies the shared write gate: empty/whitespace event
// ids are rejected with a 400 and the canonical message; a non-empty id passes.
func TestRequireEventID(t *testing.T) {
	for _, tt := range []struct {
		name    string
		eventID string
		wantOK  bool
	}{
		{name: "empty", eventID: "", wantOK: false},
		{name: "whitespace", eventID: "   ", wantOK: false},
		{name: "non-empty", eventID: "ev1", wantOK: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ok := requireEventID(rec, tt.eventID)
			if ok != tt.wantOK {
				t.Errorf("requireEventID(%q) = %v, want %v", tt.eventID, ok, tt.wantOK)
			}
			if tt.wantOK {
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d, want 200", rec.Code)
				}
				return
			}
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if body := strings.TrimSpace(rec.Body.String()); body != "an event must be selected before recording attendance" {
				t.Errorf("body = %q, want canonical message", body)
			}
		})
	}
}
