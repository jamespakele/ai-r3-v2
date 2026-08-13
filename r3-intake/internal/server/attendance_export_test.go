package server

import (
	"reflect"
	"testing"
)

// TestExportStatus verifies the stored status select value maps to the
// title-case display string used in the CSV export, and that unknown/empty
// values render as empty.
func TestExportStatus(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"present", "present", "Present"},
		{"absent", "absent", "Absent"},
		{"excused", "excused", "Excused"},
		{"walk in", "walk_in", "Walk-in"},
		{"empty", "", ""},
		{"unknown", "foo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exportStatus(tt.in); got != tt.want {
				t.Errorf("exportStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSummaryCSVRow verifies the trailing summary row math and shape: check-in
// counting, unique-participant counting, and the avg-rate formula with integer
// truncation, plus the guard against division by zero on empty input.
func TestSummaryCSVRow(t *testing.T) {
	tests := []struct {
		name string
		rows []ExportRow
		want string
	}{
		{
			name: "empty rows no panic",
			rows: nil,
			want: "Summary: 0 check-ins, 0 unique participants, 0% avg rate",
		},
		{
			name: "present and walk-in count as check-ins",
			rows: []ExportRow{
				{ParticipantName: "Alice", Date: "2026-08-01", Status: "present"},
				{ParticipantName: "Alice", Date: "2026-08-02", Status: "walk_in"},
				{ParticipantName: "Bob", Date: "2026-08-01", Status: "absent"},
				{ParticipantName: "Carol", Date: "2026-08-01", Status: "excused"},
			},
			// 2 check-ins, 3 unique, 1 present over 3 unique x 2 days = 16%.
			want: "Summary: 2 check-ins, 3 unique participants, 16% avg rate",
		},
		{
			name: "duplicate names collapse and empty names ignored",
			rows: []ExportRow{
				{ParticipantName: "Alice", Date: "2026-08-01", Status: "present"},
				{ParticipantName: "Alice", Date: "2026-08-02", Status: "present"},
				{ParticipantName: "Bob", Date: "2026-08-01", Status: "present"},
				{ParticipantName: "", Date: "2026-08-01", Status: "present"},
			},
			// 4 check-ins, 2 unique (Alice, Bob), 4 present over 2 x 2 days = 100%.
			want: "Summary: 4 check-ins, 2 unique participants, 100% avg rate",
		},
		{
			name: "rate integer truncation",
			rows: []ExportRow{
				{ParticipantName: "Alice", Date: "2026-08-01", Status: "present"},
				{ParticipantName: "Alice", Date: "2026-08-02", Status: "absent"},
				{ParticipantName: "Bob", Date: "2026-08-01", Status: "absent"},
			},
			// 1 present / (2 unique x 2 days) = 25%.
			want: "Summary: 1 check-ins, 2 unique participants, 25% avg rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summaryCSVRow(tt.rows)
			if len(got) != 8 {
				t.Fatalf("summary row length = %d, want 8", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("summary first cell = %q, want %q", got[0], tt.want)
			}
			for i := 1; i < len(got); i++ {
				if got[i] != "" {
					t.Errorf("summary col %d = %q, want empty", i, got[i])
				}
			}
		})
	}
}

// TestExportCSVRecords_Header asserts the CSV table's first row is the exact
// 8-column header, order-sensitive.
func TestExportCSVRecords_Header(t *testing.T) {
	records := exportCSVRecords(nil)
	want := []string{"Participant", "Site", "Event", "Date", "Status", "Recorded By", "Check-in Time", "Note"}
	if !reflect.DeepEqual(records[0], want) {
		t.Errorf("header = %v, want %v", records[0], want)
	}
}

// TestExportCSVRecords_FieldFormatting asserts one row per status renders the
// title-cased status in column 4 and passes check-in time / note / recorder
// through verbatim, including comma-containing and walk_in values.
func TestExportCSVRecords_FieldFormatting(t *testing.T) {
	rows := []ExportRow{
		{ParticipantName: "Alice", Date: "2026-08-01", Status: "present", CheckInTime: "2026-08-01 20:30:00.000Z", Note: "on time", RecordedByName: "Admin One"},
		{ParticipantName: "Bob", Date: "2026-08-02", Status: "walk_in", CheckInTime: "", Note: "late, note with, comma", RecordedByName: "CM One"},
		{ParticipantName: "Carol", Date: "2026-08-03", Status: "excused", CheckInTime: "", Note: "", RecordedByName: ""},
		{ParticipantName: "Dave", Date: "2026-08-04", Status: "absent", CheckInTime: "", Note: "", RecordedByName: ""},
	}

	records := exportCSVRecords(rows)

	statusByRow := map[int]string{1: "Present", 2: "Walk-in", 3: "Excused", 4: "Absent"}
	for idx, want := range statusByRow {
		if got := records[idx][4]; got != want {
			t.Errorf("row %d status = %q, want %q", idx, got, want)
		}
	}

	if got := records[1][7]; got != "on time" {
		t.Errorf("row1 note = %q, want 'on time'", got)
	}
	if got := records[1][6]; got != "2026-08-01 20:30:00.000Z" {
		t.Errorf("row1 check-in time = %q, want verbatim", got)
	}
	if got := records[1][5]; got != "Admin One" {
		t.Errorf("row1 recorded_by = %q, want 'Admin One'", got)
	}
	if got := records[2][7]; got != "late, note with, comma" {
		t.Errorf("row2 note = %q, want comma note", got)
	}
	if got := records[3][6]; got != "" {
		t.Errorf("row3 check-in time = %q, want empty", got)
	}
	if got := records[3][5]; got != "" {
		t.Errorf("row3 recorded_by = %q, want empty", got)
	}
}

// TestExportCSVRecords_Empty asserts the pure builder returns exactly two rows
// for empty input: the header row and the empty summary row.
func TestExportCSVRecords_Empty(t *testing.T) {
	records := exportCSVRecords(nil)
	if len(records) != 2 {
		t.Fatalf("records length = %d, want 2", len(records))
	}
	wantSum := "Summary: 0 check-ins, 0 unique participants, 0% avg rate"
	if records[1][0] != wantSum {
		t.Errorf("summary first cell = %q, want %q", records[1][0], wantSum)
	}
	for i := 1; i < len(records[1]); i++ {
		if records[1][i] != "" {
			t.Errorf("summary col %d = %q, want empty", i, records[1][i])
		}
	}
}
