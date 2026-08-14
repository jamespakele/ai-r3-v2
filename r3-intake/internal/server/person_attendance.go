package server

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	mcpmod "r3-intake/internal/mcp"

	"github.com/pocketbase/pocketbase/core"
)

// PersonAttendanceView is the view model for the per-participant attendance
// history page (FR15/FR16). It renders a monthly calendar grid plus per-person
// stats. Consumed by the "person-attendance" and "person-attendance-calendar"
// templates.
type PersonAttendanceView struct {
	UserName   string
	Role       string
	IsAdmin    bool
	IntakeID   string
	IntakeName string
	SiteName   string
	Month      string // "YYYY-MM" visible month
	PrevMonth  string // "YYYY-MM"
	NextMonth  string // "YYYY-MM"
	Today      string // "YYYY-MM-DD" HST
	Weekdays   []string
	Weeks      [][]PersonDayCell
	Stats      PersonStats
	Legend     []PersonLegendItem
	EventID    string // selected event ("" = none)
	Events     []Event
	// EventRequired reports that no event is selected, so day-cell writes must
	// be gated behind an event selection.
	EventRequired bool
}

// PersonDayCell is a single date cell in the monthly calendar grid.
type PersonDayCell struct {
	Date         string // "YYYY-MM-DD"
	DayNum       string // "1".."31"
	Status       string // "" | "present" | "absent" | "excused" | "walk_in"
	StatusLabel  string // "" | "Present" | ... (via exportStatus)
	IsToday      bool
	IsOtherMonth bool // leading/trailing blank cells from adjacent months
	HasRecord    bool
}

// PersonStats holds the per-person attendance stats for the visible month.
type PersonStats struct {
	PresentCount int    // status in {present, walk_in}
	TotalDays    int    // total attendance records in visible month
	Rate         int    // percent 0-100 (0 when TotalDays==0)
	RateColor    string // "green" | "red" (green >=50, red <50)
	Streak       int    // consecutive present days ending today or most recent present
	HasRecords   bool
}

// PersonLegendItem is one entry in the calendar legend.
type PersonLegendItem struct {
	Status string
	Label  string
}

// PersonDayDetailView is the view model for the day-detail modal fragment
// (FR17). Consumed by the "person-attendance-day" template.
type PersonDayDetailView struct {
	IntakeID      string
	Date          string
	HasRecord     bool
	Status        string
	StatusOptions []PersonStatusOption
	EventName     string
	RecordedBy    string
	CheckInTime   string
	Note          string
	Error         string
	EventID       string
	Events        []Event
}

// PersonStatusOption is one option in the status dropdown.
type PersonStatusOption struct {
	Value    string
	Label    string
	Selected bool
}

// personAttendanceRecord is a lightweight view of one attendance record used
// by the pure stats/grid helpers (no *Server/pb dependency).
type personAttendanceRecord struct {
	Date   string
	Status string
}

// loadPersonAttendanceIntake loads the intake record for the per-person
// attendance routes and enforces authorization: case managers may only access
// intakes assigned to them; admins may access any. On failure it writes the
// HTTP error and returns ok=false.
func (s *Server) loadPersonAttendanceIntake(w http.ResponseWriter, r *http.Request, u *sessionUser, id string) (*core.Record, bool) {
	rec, err := s.findIntake(id)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	if u.Role == "case_manager" && rec.GetString("assigned_to") != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	return rec, true
}

// handlePersonAttendance renders the per-participant attendance history page
// (FR15/FR16). Wrapped in requireAuth, so u is non-nil.
func (s *Server) handlePersonAttendance(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	intake, ok := s.loadPersonAttendanceIntake(w, r, u, r.PathValue("id"))
	if !ok {
		return
	}
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().In(hst).Format("2006-01")
	} else if _, err := time.Parse("2006-01", month); err != nil {
		month = time.Now().In(hst).Format("2006-01")
	}
	eventID := strings.TrimSpace(r.URL.Query().Get("event"))
	view := s.buildPersonAttendanceView(u, intake, month, eventID)
	_ = s.tpl.ExecuteTemplate(w, "person-attendance", view)
}

// buildPersonAttendanceView assembles the full view model for a participant
// and month: loads the month's attendance records, builds the calendar grid,
// and computes the stats. Shared by the full page and the calendar fragment.
func (s *Server) buildPersonAttendanceView(u *sessionUser, intake *core.Record, month, eventID string) PersonAttendanceView {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		t = time.Now().In(hst)
	}
	year, m := t.Year(), t.Month()
	first := time.Date(year, m, 1, 0, 0, 0, 0, hst)
	last := time.Date(year, m+1, 0, 0, 0, 0, 0, hst)
	firstStr := first.Format("2006-01-02")
	lastStr := last.Format("2006-01-02")

	events, _ := s.loadEvents("")

	records := map[string]string{}
	var recList []personAttendanceRecord
	if attCol, err := s.attendanceCollection(); err == nil {
		filter := fmt.Sprintf("intake='%s' && date>='%s' && date<='%s'",
			mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(firstStr), mcpmod.EscapeFilter(lastStr))
		if eventID != "" {
			filter = fmt.Sprintf("intake='%s' && event='%s' && date>='%s' && date<='%s'",
				mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID),
				mcpmod.EscapeFilter(firstStr), mcpmod.EscapeFilter(lastStr))
		}
		recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "date", 1000, 0)
		if err == nil {
			for _, rec := range recs {
				d := rec.GetString("date")
				st := rec.GetString("status")
				records[d] = st
				recList = append(recList, personAttendanceRecord{Date: d, Status: st})
			}
		}
	}

	today := time.Now().In(hst).Format("2006-01-02")
	weeks := buildMonthGrid(year, m, records, today)
	stats := computePersonStats(recList, today)

	return PersonAttendanceView{
		UserName:   u.Name,
		Role:       u.Role,
		IsAdmin:    u.Role == "admin",
		IntakeID:   intake.Id,
		IntakeName: intake.GetString("name"),
		SiteName:   s.nameFor("sites", intake.GetString("site")),
		Month:      month,
		PrevMonth:  t.AddDate(0, -1, 0).Format("2006-01"),
		NextMonth:  t.AddDate(0, 1, 0).Format("2006-01"),
		Today:      today,
		Weekdays:   []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		Weeks:      weeks,
		Stats:      stats,
		Legend: []PersonLegendItem{
			{Status: "present", Label: "Present"},
			{Status: "absent", Label: "Absent"},
			{Status: "excused", Label: "Excused"},
			{Status: "walk_in", Label: "Walk-in"},
		},
		EventID:       eventID,
		Events:        events,
		EventRequired: eventID == "",
	}
}

// renderPersonAttendanceCalendar renders the calendar + stats fragment (the
// HTMX swap target for POST save/delete responses).
func (s *Server) renderPersonAttendanceCalendar(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, month, eventID string) {
	view := s.buildPersonAttendanceView(u, intake, month, eventID)
	_ = s.tpl.ExecuteTemplate(w, "person-attendance-calendar", view)
}

// handlePersonAttendanceDay serves the day-detail modal fragment (GET) or
// saves/updates an attendance record (POST) for a single date (FR17).
func (s *Server) handlePersonAttendanceDay(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	intake, ok := s.loadPersonAttendanceIntake(w, r, u, r.PathValue("id"))
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handlePersonAttendanceDayGet(w, r, intake)
	case http.MethodPost:
		s.handlePersonAttendanceDaySave(w, r, u, intake)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePersonAttendanceDayGet renders the day-detail fragment for a date,
// scoped to the selected event.
func (s *Server) handlePersonAttendanceDayGet(w http.ResponseWriter, r *http.Request, intake *core.Record) {
	date := r.URL.Query().Get("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	eventID := strings.TrimSpace(r.URL.Query().Get("event"))
	view := s.buildPersonDayDetailView(intake, date, eventID, "")
	_ = s.tpl.ExecuteTemplate(w, "person-attendance-day", view)
}

// buildPersonDayDetailView loads the existing (event, intake, date) record, if
// any, and builds the day-detail view model. errMsg is surfaced in the
// fragment.
func (s *Server) buildPersonDayDetailView(intake *core.Record, date, eventID, errMsg string) PersonDayDetailView {
	view := PersonDayDetailView{
		IntakeID: intake.Id,
		Date:     date,
		Error:    errMsg,
		EventID:  eventID,
	}
	events, _ := s.loadEvents("")
	view.Events = events
	if attCol, err := s.attendanceCollection(); err == nil {
		filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
			mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
		recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
		if err == nil && len(recs) > 0 {
			rec := recs[0]
			view.HasRecord = true
			view.Status = rec.GetString("status")
			view.EventID = rec.GetString("event")
			view.EventName = s.nameFor("events", rec.GetString("event"))
			view.RecordedBy = s.nameFor("users", rec.GetString("recorded_by"))
			view.CheckInTime = rec.GetString("check_in_time")
			view.Note = rec.GetString("note")
		}
	}
	view.StatusOptions = statusOptions(view.Status)
	return view
}

// handlePersonAttendanceDaySave creates or updates the attendance record for
// (intake, date) and re-renders the calendar fragment. On validation error it
// re-renders the day fragment with an error message and a 400 status.
func (s *Server) handlePersonAttendanceDaySave(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record) {
	date := r.FormValue("date")
	status := r.FormValue("status")
	note := strings.TrimSpace(r.FormValue("note"))
	eventID := strings.TrimSpace(r.FormValue("event_id"))

	if _, err := time.Parse("2006-01-02", date); err != nil {
		s.renderDayError(w, intake, date, eventID, "Invalid date")
		return
	}
	if !requireEventID(w, eventID) {
		return
	}
	if !validAttendanceStatus(status) {
		s.renderDayError(w, intake, date, eventID, "Invalid status")
		return
	}
	if len(note) > 500 {
		s.renderDayError(w, intake, date, eventID, "Note must be 500 characters or fewer")
		return
	}

	attCol, err := s.attendanceCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
		mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(recs) > 0 {
		rec := recs[0]
		rec.Set("status", status)
		rec.Set("note", note)
		rec.Set("event", eventID)
		rec.Set("recorded_by", u.ID)
		if err := s.pb.Save(rec); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		rec := core.NewRecord(attCol)
		rec.Set("intake", intake.Id)
		rec.Set("event", eventID)
		rec.Set("date", date)
		rec.Set("status", status)
		rec.Set("note", note)
		rec.Set("site", intake.GetString("site"))
		rec.Set("recorded_by", u.ID)
		if err := s.pb.Save(rec); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	month := date[:7] // YYYY-MM
	s.renderPersonAttendanceCalendar(w, r, u, intake, month, eventID)
}

// renderDayError writes the day-detail fragment with an error message and a
// 400 status.
func (s *Server) renderDayError(w http.ResponseWriter, intake *core.Record, date, eventID, msg string) {
	view := s.buildPersonDayDetailView(intake, date, eventID, msg)
	w.WriteHeader(http.StatusBadRequest)
	_ = s.tpl.ExecuteTemplate(w, "person-attendance-day", view)
}

// handlePersonAttendanceDayDelete deletes the attendance record for
// (intake, date) and re-renders the calendar fragment. Idempotent: deleting a
// date with no record is a no-op success.
func (s *Server) handlePersonAttendanceDayDelete(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	intake, ok := s.loadPersonAttendanceIntake(w, r, u, r.PathValue("id"))
	if !ok {
		return
	}
	date := r.FormValue("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	eventID := strings.TrimSpace(r.FormValue("event_id"))
	if attCol, err := s.attendanceCollection(); err == nil {
		filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
			mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
		recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
		if err == nil && len(recs) > 0 {
			if derr := s.pb.Delete(recs[0]); derr != nil {
				http.Error(w, "delete failed: "+derr.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	month := date[:7] // YYYY-MM
	s.renderPersonAttendanceCalendar(w, r, u, intake, month, eventID)
}

// validAttendanceStatus reports whether s is a valid attendance status value.
func validAttendanceStatus(s string) bool {
	switch s {
	case "present", "absent", "excused", "walk_in":
		return true
	}
	return false
}

// statusOptions returns the four attendance statuses as dropdown options, with
// the current status selected.
func statusOptions(current string) []PersonStatusOption {
	order := []struct{ v, l string }{
		{"present", "Present"},
		{"absent", "Absent"},
		{"excused", "Excused"},
		{"walk_in", "Walk-in"},
	}
	opts := make([]PersonStatusOption, 0, len(order))
	for _, o := range order {
		opts = append(opts, PersonStatusOption{Value: o.v, Label: o.l, Selected: o.v == current})
	}
	return opts
}

// computePersonStats computes the per-person stats for a set of attendance
// records. Pure (no *Server/pb dependency) so unit tests can call it directly.
func computePersonStats(records []personAttendanceRecord, today string) PersonStats {
	total := len(records)
	present := 0
	var presentDates []string
	for _, rec := range records {
		if rec.Status == "present" || rec.Status == "walk_in" {
			present++
			presentDates = append(presentDates, rec.Date)
		}
	}
	rate := 0
	if total > 0 {
		rate = present * 100 / total
	}
	color := "red"
	if rate >= 50 {
		color = "green"
	}
	streak := 0
	if len(presentDates) > 0 {
		sort.Strings(presentDates)
		streak = 1
		prev, _ := time.Parse("2006-01-02", presentDates[len(presentDates)-1])
		for i := len(presentDates) - 2; i >= 0; i-- {
			cur, _ := time.Parse("2006-01-02", presentDates[i])
			if prev.Sub(cur).Hours() == 24 {
				streak++
				prev = cur
			} else {
				break
			}
		}
	}
	return PersonStats{
		PresentCount: present,
		TotalDays:    total,
		Rate:         rate,
		RateColor:    color,
		Streak:       streak,
		HasRecords:   total > 0,
	}
}

// buildMonthGrid builds the 7-column calendar grid for a month, padded with
// leading/trailing cells from adjacent months so the grid is a full rectangle
// starting on Sunday. Pure so unit tests can call it directly.
func buildMonthGrid(year int, month time.Month, records map[string]string, today string) [][]PersonDayCell {
	first := time.Date(year, month, 1, 0, 0, 0, 0, hst)
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, hst)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	end := last.AddDate(0, 0, int(time.Saturday-last.Weekday()))

	var cells []PersonDayCell
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		status := records[date]
		cell := PersonDayCell{
			Date:         date,
			DayNum:       "",
			Status:       status,
			StatusLabel:  exportStatus(status),
			IsToday:      date == today,
			IsOtherMonth: d.Month() != month,
			HasRecord:    status != "",
		}
		if d.Month() == month {
			cell.DayNum = strconv.Itoa(d.Day())
		}
		cells = append(cells, cell)
	}

	var weeks [][]PersonDayCell
	for i := 0; i < len(cells); i += 7 {
		end := i + 7
		if end > len(cells) {
			end = len(cells)
		}
		weeks = append(weeks, cells[i:end])
	}
	return weeks
}
