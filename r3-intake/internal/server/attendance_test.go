package server

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"r3-intake/internal/assets"
)

func TestComputeSummary(t *testing.T) {
	tests := []struct {
		name  string
		rows  []MatrixRow
		days  int
		want  MatrixSummary
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
