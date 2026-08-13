package server

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"r3-intake/internal/assets"
)

func TestComputePersonStats(t *testing.T) {
	tests := []struct {
		name    string
		records []personAttendanceRecord
		want    PersonStats
	}{
		{
			name: "mixed statuses",
			records: []personAttendanceRecord{
				{Date: "2026-08-01", Status: "present"},
				{Date: "2026-08-02", Status: "walk_in"},
				{Date: "2026-08-03", Status: "absent"},
				{Date: "2026-08-04", Status: "excused"},
			},
			want: PersonStats{PresentCount: 2, TotalDays: 4, Rate: 50, RateColor: "green", Streak: 2, HasRecords: true},
		},
		{
			name:    "empty",
			records: nil,
			want:    PersonStats{PresentCount: 0, TotalDays: 0, Rate: 0, RateColor: "red", Streak: 0, HasRecords: false},
		},
		{
			name:    "rate 49 red",
			records: buildRecords(49, 51),
			want:    PersonStats{PresentCount: 49, TotalDays: 100, Rate: 49, RateColor: "red", Streak: 1, HasRecords: true},
		},
		{
			name:    "rate 50 green",
			records: buildRecords(50, 50),
			want:    PersonStats{PresentCount: 50, TotalDays: 100, Rate: 50, RateColor: "green", Streak: 1, HasRecords: true},
		},
		{
			name:    "rate 51 green",
			records: buildRecords(51, 49),
			want:    PersonStats{PresentCount: 51, TotalDays: 100, Rate: 51, RateColor: "green", Streak: 1, HasRecords: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePersonStats(tt.records, "2026-08-10")
			if got != tt.want {
				t.Errorf("computePersonStats() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// buildRecords returns presentCount present records followed by absentCount
// absent records, with consecutive dates starting 2026-08-01.
func buildRecords(presentCount, absentCount int) []personAttendanceRecord {
	start, _ := time.Parse("2006-01-02", "2026-08-01")
	var recs []personAttendanceRecord
	for i := range presentCount {
		recs = append(recs, personAttendanceRecord{Date: start.AddDate(0, 0, i*2).Format("2006-01-02"), Status: "present"})
	}
	for i := range absentCount {
		recs = append(recs, personAttendanceRecord{Date: start.AddDate(0, 0, presentCount*2+i).Format("2006-01-02"), Status: "absent"})
	}
	return recs
}

func TestComputeStreak(t *testing.T) {
	tests := []struct {
		name    string
		records []personAttendanceRecord
		today   string
		want    int
	}{
		{
			name: "consecutive ending today",
			records: []personAttendanceRecord{
				{Date: "2026-08-01", Status: "present"},
				{Date: "2026-08-02", Status: "present"},
				{Date: "2026-08-03", Status: "walk_in"},
			},
			today: "2026-08-03",
			want:  3,
		},
		{
			name: "most recent present not today",
			records: []personAttendanceRecord{
				{Date: "2026-08-01", Status: "present"},
				{Date: "2026-08-02", Status: "present"},
				{Date: "2026-08-03", Status: "present"},
			},
			today: "2026-08-05",
			want:  3,
		},
		{
			name: "gap breaks streak",
			records: []personAttendanceRecord{
				{Date: "2026-08-01", Status: "present"},
				{Date: "2026-08-02", Status: "present"},
				{Date: "2026-08-04", Status: "present"},
			},
			today: "2026-08-04",
			want:  1,
		},
		{
			name: "no present",
			records: []personAttendanceRecord{
				{Date: "2026-08-01", Status: "absent"},
				{Date: "2026-08-02", Status: "excused"},
			},
			today: "2026-08-02",
			want:  0,
		},
		{
			name:    "empty",
			records: nil,
			today:   "2026-08-02",
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computePersonStats(tt.records, tt.today).Streak
			if got != tt.want {
				t.Errorf("streak = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildMonthGrid(t *testing.T) {
	// August 2026: 31 days. Compute expected padding from the actual weekday.
	first := time.Date(2026, time.August, 1, 0, 0, 0, 0, hst)
	last := time.Date(2026, time.August, 31, 0, 0, 0, 0, hst)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	end := last.AddDate(0, 0, int(time.Saturday-last.Weekday()))

	records := map[string]string{
		"2026-08-01": "present",
		"2026-08-15": "absent",
	}
	today := "2026-08-15"

	weeks := buildMonthGrid(2026, time.August, records, today)

	// Every row must have exactly 7 cells.
	for i, row := range weeks {
		if len(row) != 7 {
			t.Errorf("row %d has %d cells, want 7", i, len(row))
		}
	}

	// First cell is the Sunday on/before the 1st; last cell is the Saturday
	// on/after the last day.
	if got := weeks[0][0].Date; got != start.Format("2006-01-02") {
		t.Errorf("first cell date = %s, want %s", got, start.Format("2006-01-02"))
	}
	lastRow := weeks[len(weeks)-1]
	if got := lastRow[6].Date; got != end.Format("2006-01-02") {
		t.Errorf("last cell date = %s, want %s", got, end.Format("2006-01-02"))
	}

	// IsToday only on the today date.
	for _, row := range weeks {
		for _, cell := range row {
			if cell.IsToday && cell.Date != today {
				t.Errorf("IsToday set on %s, want only %s", cell.Date, today)
			}
			if !cell.IsToday && cell.Date == today {
				t.Errorf("IsToday not set on %s", cell.Date)
			}
		}
	}

	// IsOtherMonth on leading/trailing padding, not on visible-month days.
	for _, row := range weeks {
		for _, cell := range row {
			inMonth := cell.Date >= "2026-08-01" && cell.Date <= "2026-08-31"
			if cell.IsOtherMonth == inMonth {
				t.Errorf("cell %s IsOtherMonth=%v, want %v", cell.Date, cell.IsOtherMonth, !inMonth)
			}
		}
	}

	// Status/status label populated from the records map.
	byDate := map[string]PersonDayCell{}
	for _, row := range weeks {
		for _, cell := range row {
			byDate[cell.Date] = cell
		}
	}
	if c := byDate["2026-08-01"]; !c.HasRecord || c.Status != "present" || c.StatusLabel != "Present" {
		t.Errorf("2026-08-01 cell = %+v, want present/Present", c)
	}
	if c := byDate["2026-08-15"]; !c.HasRecord || c.Status != "absent" || c.StatusLabel != "Absent" {
		t.Errorf("2026-08-15 cell = %+v, want absent/Absent", c)
	}
	if c := byDate["2026-08-10"]; c.HasRecord {
		t.Errorf("2026-08-10 cell HasRecord = true, want false")
	}
}

func TestPersonAttendanceTemplateRenders(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := PersonAttendanceView{
		UserName:   "Admin One",
		Role:       "admin",
		IsAdmin:    true,
		IntakeID:   "i1",
		IntakeName: "Alice",
		SiteName:   "Kona",
		Month:      "2026-08",
		PrevMonth:  "2026-07",
		NextMonth:  "2026-09",
		Today:      "2026-08-15",
		Weekdays:   []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		Weeks: [][]PersonDayCell{
			{
				{Date: "2026-08-01", DayNum: "1", Status: "present", StatusLabel: "Present", HasRecord: true},
				{Date: "2026-08-02", DayNum: "2", Status: "absent", StatusLabel: "Absent", HasRecord: true},
				{Date: "2026-08-03", DayNum: "3", Status: "walk_in", StatusLabel: "Walk-in", HasRecord: true},
				{Date: "2026-08-04", DayNum: "4", Status: "excused", StatusLabel: "Excused", HasRecord: true},
				{Date: "2026-08-05", DayNum: "5", IsToday: true},
				{Date: "2026-08-06", DayNum: "6"},
				{Date: "2026-08-07", DayNum: "7"},
			},
		},
		Stats: PersonStats{PresentCount: 2, TotalDays: 4, Rate: 50, RateColor: "green", Streak: 2, HasRecords: true},
		Legend: []PersonLegendItem{
			{Status: "present", Label: "Present"},
			{Status: "absent", Label: "Absent"},
			{Status: "excused", Label: "Excused"},
			{Status: "walk_in", Label: "Walk-in"},
		},
	}

	// Full page.
	var full bytes.Buffer
	if err := tpl.ExecuteTemplate(&full, "person-attendance", view); err != nil {
		t.Fatalf("render person-attendance: %v", err)
	}
	fullOut := full.String()
	for _, want := range []string{"Alice", "Kona", "2 of 4 days (50%)", "Current streak: 2", "Present", "Absent", "Walk-in", "Excused"} {
		if !strings.Contains(fullOut, want) {
			t.Errorf("person-attendance output missing %q", want)
		}
	}

	// Calendar fragment.
	var cal bytes.Buffer
	if err := tpl.ExecuteTemplate(&cal, "person-attendance-calendar", view); err != nil {
		t.Fatalf("render person-attendance-calendar: %v", err)
	}
	calOut := cal.String()
	for _, want := range []string{"2 of 4 days (50%)", "Current streak: 2", "2026-07", "2026-09", "Present", "Absent"} {
		if !strings.Contains(calOut, want) {
			t.Errorf("person-attendance-calendar output missing %q", want)
		}
	}

	// Day detail fragment with a record.
	day := PersonDayDetailView{
		IntakeID:   "i1",
		Date:       "2026-08-01",
		HasRecord:  true,
		Status:     "present",
		StatusOptions: []PersonStatusOption{
			{Value: "present", Label: "Present", Selected: true},
			{Value: "absent", Label: "Absent"},
			{Value: "excused", Label: "Excused"},
			{Value: "walk_in", Label: "Walk-in"},
		},
		EventName:   "Morning Program",
		RecordedBy:  "Admin One",
		CheckInTime: "2026-08-01 20:30:00",
		Note:        "on time",
	}
	var dayBuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&dayBuf, "person-attendance-day", day); err != nil {
		t.Fatalf("render person-attendance-day: %v", err)
	}
	dayOut := dayBuf.String()
	for _, want := range []string{"Morning Program", "Admin One", "2026-08-01 20:30:00", "on time", "Delete this attendance record?"} {
		if !strings.Contains(dayOut, want) {
			t.Errorf("person-attendance-day output missing %q", want)
		}
	}

	// Day detail fragment empty state.
	empty := PersonDayDetailView{
		IntakeID: "i1",
		Date:     "2026-08-10",
		StatusOptions: []PersonStatusOption{
			{Value: "present", Label: "Present", Selected: true},
			{Value: "absent", Label: "Absent"},
			{Value: "excused", Label: "Excused"},
			{Value: "walk_in", Label: "Walk-in"},
		},
	}
	var emptyBuf bytes.Buffer
	if err := tpl.ExecuteTemplate(&emptyBuf, "person-attendance-day", empty); err != nil {
		t.Fatalf("render person-attendance-day empty: %v", err)
	}
	if !strings.Contains(emptyBuf.String(), "No attendance recorded") {
		t.Errorf("empty day output missing empty-state text")
	}
}
