package server

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	mcpmod "r3-intake/internal/mcp"

	"github.com/pocketbase/pocketbase/core"
)

// MatrixViewData is the view model for the attendance matrix screen.
type MatrixViewData struct {
	UserName string
	Role     string
	IsAdmin  bool
	SiteID   string   // resolved site ("" = All locations, admin only)
	SiteName string   // display name for the filter bar
	DateFrom string   // YYYY-MM-DD
	DateTo   string   // YYYY-MM-DD
	Dates    []string // YYYY-MM-DD, inclusive
	Rows     []MatrixRow
	Sites    []Site
	Events   []Event
	Summary  MatrixSummary
	EventID  string // optional event filter ("" = none)
	// EventRequired reports that no event is selected, so the matrix must
	// disable toggling.
	EventRequired bool
	// NoEvents reports that there are no active events, so the template can
	// render the "Create an Event" empty state.
	NoEvents bool
	// EventLocation is the display-only location name of the currently selected
	// event. It is empty when no event is selected or the event has no site.
	EventLocation string
	// EventStartLabel / EventEndLabel are the selected event's date range
	// formatted for display (e.g. "Mar 1" / "Apr 15, 2026"). They are empty
	// when no event is selected or the event has no dates. Used to render
	// the "Event dates" label.
	EventStartLabel string
	EventEndLabel   string
	// DatesLabel is parallel to Dates, formatted as "M/D/YY" for the grid
	// column headers.
	DatesLabel []string
}

// MatrixRow is one participant row in the matrix.
type MatrixRow struct {
	IntakeID     string
	Name         string
	Cells        []MatrixCell // one per date, in Dates order
	TotalDays    int
	PresentCount int
	WalkInCount  int
	LastPresent string // YYYY-MM-DD or ""
}

// MatrixSummary holds the aggregate stat cards below the matrix, computed
// from the same rows that rendered the grid so cards always match the matrix.
type MatrixSummary struct {
	TotalCheckIns      int
	ActiveParticipants int
	AvgRate            int // 0–100, truncated (integer division)
}

// MatrixCell is a single date cell for a participant.
type MatrixCell struct {
	IntakeID string
	Date     string
	Status   string // "present" (here) or "" (not here)
	SiteID   string
	EventID  string
	From     string
	To       string
	Disabled bool // true when the participant has no location; attendance cannot be toggled
}

// attendanceCollection returns the attendance collection.
func (s *Server) attendanceCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("attendance")
}

// handleMatrix renders the attendance matrix screen. It is wrapped with
// requireAuth in the mux, so an authenticated session is guaranteed here.
func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)

	events, err := s.loadEvents()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	from, to, eventID, dates := s.parseMatrixFilters(r, events)

	rows, err := s.loadMatrixRows(u, dates, eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	eventLocation := ""
	eventStartDate := ""
	eventEndDate := ""
	if eventID != "" {
		for _, ev := range events {
			if ev.ID == eventID {
				eventLocation = s.nameFor("sites", ev.SiteID)
				eventStartDate = ev.StartDate
				eventEndDate = ev.EndDate
				break
			}
		}
	}

	eventStartLabel := formatEventStart(eventStartDate)
	eventEndLabel := formatEventEnd(eventEndDate)
	datesLabel := make([]string, len(dates))
	for i, d := range dates {
		datesLabel[i] = formatGridDate(d)
	}

	// Non-admin filter bar still shows the resolved site name.
	_, siteName := s.resolveSite(u, "")

	view := MatrixViewData{
		UserName:      u.Name,
		Role:          u.Role,
		IsAdmin:       u.Role == "admin",
		SiteName:      siteName,
		DateFrom:      from,
		DateTo:        to,
		Dates:         dates,
		Rows:          rows,
		Events:        events,
		Summary:       computeSummary(rows, len(dates)),
		EventID:       eventID,
		EventRequired: eventID == "",
		NoEvents:       len(events) == 0,
		EventLocation:   eventLocation,
		EventStartLabel: eventStartLabel,
		EventEndLabel:   eventEndLabel,
		DatesLabel:      datesLabel,
	}

	if r.Header.Get("HX-Request") == "true" {
		_ = s.tpl.ExecuteTemplate(w, "matrix-content", view)
		return
	}
	_ = s.tpl.ExecuteTemplate(w, "matrix", view)
}

// parseMatrixFilters derives the effective attendance filters from the
// request query, applying the same defaults and validation used by the
// matrix: a 14-day window (today and the prior 13 days), inverted ranges
// swapped, and ranges capped at 30 days. When the user did not explicitly
// provide a valid from/to range and an event is in effect, the range
// auto-scopes to that event's start_date -> end_date (full span, no cap).
// The event filter is read from the query key "event" (matrix filter bar)
// and falls back to "event_id" (toggle forms).
func (s *Server) parseMatrixFilters(r *http.Request, events []Event) (from, to, eventID string, dates []string) {
	eventID = strings.TrimSpace(r.URL.Query().Get("event"))
	if eventID == "" {
		eventID = strings.TrimSpace(r.URL.Query().Get("event_id"))
	}

	// Parse and validate from/to.
	now := time.Now().In(hst)
	defTo := now.Format("2006-01-02")
	defFrom := now.AddDate(0, 0, -13).Format("2006-01-02")

	from = strings.TrimSpace(r.URL.Query().Get("from"))
	to = strings.TrimSpace(r.URL.Query().Get("to"))
	fromT, errFrom := time.Parse("2006-01-02", from)
	toT, errTo := time.Parse("2006-01-02", to)
	explicitRange := errFrom == nil && errTo == nil
	if errFrom != nil || errTo != nil {
		from, to = defFrom, defTo
		fromT, _ = time.Parse("2006-01-02", from)
		toT, _ = time.Parse("2006-01-02", to)
	}
	if fromT.After(toT) {
		fromT, toT = toT, fromT
		from, to = fromT.Format("2006-01-02"), toT.Format("2006-01-02")
	}
	// Cap range at 30 days from the start date.
	if toT.Sub(fromT) > 30*24*time.Hour {
		toT = fromT.AddDate(0, 0, 29)
		to = toT.Format("2006-01-02")
	}

	eventID = s.effectiveEventID(eventID, events)

	// Auto-scope to the effective event's dates when the user did not
	// explicitly provide a valid from/to range. The event span is
	// authoritative: no 30-day cap applies.
	if !explicitRange && eventID != "" {
		for _, ev := range events {
			if ev.ID == eventID {
				start, errStart := time.Parse("2006-01-02", ev.StartDate)
				end, errEnd := time.Parse("2006-01-02", ev.EndDate)
				if errStart == nil && errEnd == nil && !start.After(end) {
					from, to = ev.StartDate, ev.EndDate
				}
				break
			}
		}
	}

	dates = buildDateRange(from, to)
	return from, to, eventID, dates
}

// effectiveEventID resolves the event the matrix and stat cards operate on.
// An explicit eventID wins; otherwise the first active event (loadEvents
// sorts by start_date,name) is the default; with no events it returns "".
func (s *Server) effectiveEventID(eventID string, events []Event) string {
	if eventID != "" {
		return eventID
	}
	if len(events) > 0 {
		return events[0].ID
	}
	return ""
}

// handleStats renders only the stat-cards fragment for the current filters.
// It is wrapped with requireAuth in the mux and only ever called via HTMX
// after a dot toggle, so it shares parseMatrixFilters with handleMatrix to
// guarantee the cards always match the matrix.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)

	events, err := s.loadEvents()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _, eventID, dates := s.parseMatrixFilters(r, events)

	rows, err := s.loadMatrixRows(u, dates, eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := MatrixViewData{Summary: computeSummary(rows, len(dates))}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tpl.ExecuteTemplate(w, "stat-cards", view)
}

// buildDateRange returns every YYYY-MM-DD from `from` to `to` inclusive.
func buildDateRange(from, to string) []string {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil
	}
	var out []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

// resolveSite returns the effective site ID and display name for the user.
// Admins may pick any active site or "" (All locations); case managers are
// pinned to the site derived from their assigned intakes.
func (s *Server) resolveSite(u *sessionUser, param string) (string, string) {
	sites, err := s.loadSites(false)
	if err != nil {
		return "", ""
	}
	if u.Role == "admin" {
		if param == "" {
			return "", "All locations"
		}
		for _, st := range sites {
			if st.ID == param {
				return st.ID, st.Name
			}
		}
		return "", "All locations"
	}
	// case_manager: derive site from assigned intakes. Each intake's home
	// event determines the site: resolve the event's site and count by site so
	// the derived value stays a site id (callers use it as a site
	// filter/guard). The matrix is event-scoped, but this function's contract
	// is a site, so we resolve through the event rather than keying by event.
	counts := map[string]int{}
	col, err := s.intakeCollection()
	if err == nil {
		filter := fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
		recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "name", 1000, 0)
		if err == nil {
			eventsCol, eerr := s.eventsCollection()
			for _, rec := range recs {
				eventID := rec.GetString("event")
				if eventID == "" || eerr != nil {
					continue
				}
				ev, err := s.pb.FindRecordById(eventsCol.Id, eventID)
				if err != nil {
					continue
				}
				sid := ev.GetString("site")
				if sid != "" {
					counts[sid]++
				}
			}
		}
	}
	if len(counts) > 0 {
		best := ""
		bestN := -1
		for sid, n := range counts {
			if n > bestN {
				best, bestN = sid, n
			}
		}
		for _, st := range sites {
			if st.ID == best {
				return st.ID, st.Name
			}
		}
	}
	// Fallback: first active site.
	if len(sites) > 0 {
		return sites[0].ID, sites[0].Name
	}
	return "", ""
}

// loadMatrixRows builds the participant rows and fills cells from attendance.
func (s *Server) loadMatrixRows(u *sessionUser, dates []string, eventID string) ([]MatrixRow, error) {
	intakeCol, err := s.intakeCollection()
	if err != nil {
		return nil, err
	}
	attCol, err := s.attendanceCollection()
	if err != nil {
		return nil, err
	}

	// Participants: ALWAYS the full site/role-scoped roster, independent of the
	// selected event (AC #1). The event only scopes the attendance map below; it
	// must never change which participants are listed.
	eventSite := ""
	if eventID != "" {
		eventsCol, err := s.eventsCollection()
		if err != nil {
			return nil, err
		}
		if eventRec, err := s.pb.FindRecordById(eventsCol.Id, eventID); err == nil {
			eventSite = eventRec.GetString("site")
		}
	}
	var intakeFilter string
	switch {
	case u.Role == "case_manager":
		intakeFilter = fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
	default:
		intakeFilter = "1=1"
	}
	intakeRecs, err := s.pb.FindRecordsByFilter(intakeCol.Id, intakeFilter, "name", 1000, 0)
	if err != nil {
		return nil, err
	}

	// Attendance map: intakeID -> date -> status. Location is derived from the
	// event's site, so the attendance query filters by event set, never by site.
	from := dates[0]
	toDate := dates[len(dates)-1]
	eventIDs, err := s.resolveEventIDs(eventID)
	if err != nil {
		return nil, err
	}
	attMap := map[string]map[string]string{}
	if eventIDs == nil || len(eventIDs) > 0 {
		attFilter := fmt.Sprintf("date>='%s' && date<='%s'", mcpmod.EscapeFilter(from), mcpmod.EscapeFilter(toDate))
		if eventIDs != nil {
			ors := make([]string, 0, len(eventIDs))
			for _, id := range eventIDs {
				ors = append(ors, "event='"+mcpmod.EscapeFilter(id)+"'")
			}
			attFilter += " && (" + strings.Join(ors, " || ") + ")"
		}
		attRecs, err := s.pb.FindRecordsByFilter(attCol.Id, attFilter, "date", 5000, 0)
		if err != nil {
			return nil, err
		}
		for _, rec := range attRecs {
			iid := rec.GetString("intake")
			d := rec.GetString("date")
			if attMap[iid] == nil {
				attMap[iid] = map[string]string{}
			}
			attMap[iid][d] = rec.GetString("status")
		}
	}

	rows := make([]MatrixRow, 0, len(intakeRecs))
	for _, rec := range intakeRecs {
		iid := rec.Id
		cellSiteID := eventSite
		if cellSiteID == "" {
			cellSiteID = rec.GetString("event")
		}
		row := MatrixRow{
			IntakeID:  iid,
			Name:      rec.GetString("name"),
			TotalDays: len(dates),
		}
		row.Cells = make([]MatrixCell, 0, len(dates))
		for _, d := range dates {
			status := attMap[iid][d]
			row.Cells = append(row.Cells, MatrixCell{
				IntakeID: iid,
				Date:     d,
				Status:   status,
				SiteID:   cellSiteID,
				EventID:  eventID,
				From:     from,
				To:       toDate,
				Disabled: cellSiteID == "" || eventID == "",
			})
			if status == "present" {
				row.PresentCount++
				if d > row.LastPresent {
					row.LastPresent = d
				}
			}
			if status == "walk_in" {
				row.WalkInCount++
				if d > row.LastPresent {
					row.LastPresent = d
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// computeSummary aggregates the summary stat cards from the matrix rows.
// days is the number of dates in the range; when rows or days are empty, all
// stats return 0 to avoid a division-by-zero on the average rate.
func computeSummary(rows []MatrixRow, days int) MatrixSummary {
	if len(rows) == 0 || days == 0 {
		return MatrixSummary{}
	}
	s := MatrixSummary{}
	totalPresent := 0
	for _, row := range rows {
		s.TotalCheckIns += row.PresentCount + row.WalkInCount
		if row.PresentCount >= 1 {
			s.ActiveParticipants++
		}
		totalPresent += row.PresentCount
	}
	s.AvgRate = totalPresent * 100 / (len(rows) * days)
	return s
}

// formatEventStart renders an event's start date for the "Event dates" label,
// omitting the year (events are expected to fall within a single year).
func formatEventStart(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return ""
	}
	return t.Format("Jan 2")
}

// formatEventEnd renders an event's end date for the "Event dates" label,
// including the year (e.g. "Apr 15, 2026").
func formatEventEnd(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return ""
	}
	return t.Format("Jan 2, 2006")
}

// formatGridDate renders a matrix column header date compactly as "M/D/YY".
func formatGridDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return ""
	}
	return t.Format("1/2/06")
}

// Event is a flat view of an events record for templates.
type Event struct {
	ID        string
	Name      string
	SiteID    string
	StartDate string
	EndDate   string
	Status    string
}

// eventsCollection returns the events collection.
func (s *Server) eventsCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("events")
}

// loadEvents returns all active, non-deleted events.
func (s *Server) loadEvents() ([]Event, error) {
	col, err := s.eventsCollection()
	if err != nil {
		return nil, err
	}
	filter := "status='active' && deleted=false"
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "start_date,name", 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(recs))
	for _, r := range recs {
		out = append(out, Event{
			ID:        r.Id,
			Name:      r.GetString("name"),
			SiteID:    r.GetString("site"),
			StartDate: r.GetString("start_date"),
			EndDate:   r.GetString("end_date"),
			Status:    r.GetString("status"),
		})
	}
	return out, nil
}

// resolveEventIDs returns the set of event IDs to include in an attendance
// query. A specific eventID wins; otherwise nil means no event restriction
// (all active events).
func (s *Server) resolveEventIDs(eventID string) ([]string, error) {
	if eventID != "" {
		return []string{eventID}, nil
	}
	return nil, nil
}

// requireEventID returns a 400 if eventID is empty. Attendance is now
// event-scoped; every write path must select an event first.
func requireEventID(w http.ResponseWriter, eventID string) bool {
	if strings.TrimSpace(eventID) == "" {
		http.Error(w, "an event must be selected before recording attendance", http.StatusBadRequest)
		return false
	}
	return true
}

// cycleStatus returns the next status in the cycle.
// "" -> present -> "" (a simple here / not-here toggle).
func cycleStatus(current string) string {
	if current == "" {
		return "present"
	}
	return ""
}

// handleToggle handles the HTMX cell toggle POST. It cycles the attendance
// status for (intake, date) and returns the updated cell fragment (HTMX) or a
// 303 redirect back to the matrix (no-JS fallback).
func (s *Server) handleToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := s.currentSession(r)

	intakeID := strings.TrimSpace(r.FormValue("intake_id"))
	date := strings.TrimSpace(r.FormValue("date"))
	siteID := strings.TrimSpace(r.FormValue("site_id"))
	eventID := strings.TrimSpace(r.FormValue("event_id"))
	from := strings.TrimSpace(r.FormValue("from"))
	to := strings.TrimSpace(r.FormValue("to"))

	if intakeID == "" || date == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	if !requireEventID(w, eventID) {
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
	}
	if to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			http.Error(w, "invalid to", http.StatusBadRequest)
			return
		}
	}

	// Authorization: case managers may only toggle their own intakes.
	// The intake record is also the source of the effective event for
	// participants with no explicit event selected.
	intakeCol, err := s.intakeCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	intakeLoaded := false
	if u.Role == "case_manager" {
		rec, err := s.pb.FindRecordById(intakeCol.Id, intakeID)
		if err != nil || rec.GetString("assigned_to") != u.ID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		intakeLoaded = true
		if siteID == "" {
			siteID = rec.GetString("event")
		}
	}
	if siteID == "" && !intakeLoaded {
		if rec, err := s.pb.FindRecordById(intakeCol.Id, intakeID); err == nil {
			siteID = rec.GetString("event")
		}
	}

	// Attendance cannot be toggled for a participant with no assigned
	// location. The intake record is the source of truth for the event.
	if siteID == "" {
		http.Error(w, "attendance requires a location", http.StatusBadRequest)
		return
	}
	if !requireEventID(w, eventID) {
		return
	}

	attCol, err := s.attendanceCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find existing record. Event is required, so uniqueness is keyed on the
	// full (event, intake, date) tuple.
	filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
		mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var existing *core.Record
	existingStatus := ""
	if len(recs) > 0 {
		existing = recs[0]
		existingStatus = existing.GetString("status")
	}

	next := cycleStatus(existingStatus)
	now := time.Now().In(hst).Format("2006-01-02 15:04:05")

	renderStatus := next
	switch {
	case next == "" && existing != nil:
		if err := s.pb.Delete(existing); err != nil {
			log.Printf("attendance toggle delete failed: %v", err)
			renderStatus = existingStatus
		}
	case next != "" && existing == nil:
		rec := core.NewRecord(attCol)
		rec.Set("intake", intakeID)
		rec.Set("date", date)
		rec.Set("status", next)
		rec.Set("recorded_by", u.ID)
		rec.Set("check_in_time", now)
		rec.Set("event", eventID)
		if err := s.pb.Save(rec); err != nil {
			log.Printf("attendance toggle insert failed: %v", err)
			// Unique-index race: another request created the row. Re-fetch and update.
			recs, ferr := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
			if ferr == nil && len(recs) > 0 {
				recs[0].Set("status", next)
				recs[0].Set("recorded_by", u.ID)
				recs[0].Set("check_in_time", now)
				recs[0].Set("event", eventID)
				if serr := s.pb.Save(recs[0]); serr != nil {
					log.Printf("attendance toggle update-after-race failed: %v", serr)
					renderStatus = ""
				}
			} else {
				renderStatus = ""
			}
		}
	case next != "" && existing != nil:
		existing.Set("status", next)
		existing.Set("recorded_by", u.ID)
		existing.Set("check_in_time", now)
		existing.Set("event", eventID)
		if err := s.pb.Save(existing); err != nil {
			log.Printf("attendance toggle update failed: %v", err)
			renderStatus = existingStatus
		}
	}

	cell := MatrixCell{
		IntakeID: intakeID,
		Date:     date,
		Status:   renderStatus,
		SiteID:   siteID,
		EventID:  eventID,
		From:     from,
		To:       to,
		Disabled: siteID == "",
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = s.tpl.ExecuteTemplate(w, "matrix-cell", cell)
		return
	}

	// No-JS fallback: 303 redirect back to the matrix with the same filters.
	var qs []string
	if siteID != "" {
		qs = append(qs, "site="+siteID)
	}
	if from != "" {
		qs = append(qs, "from="+from)
	}
	if to != "" {
		qs = append(qs, "to="+to)
	}
	if eventID != "" {
		qs = append(qs, "event="+eventID)
	}
	url := "/attendance"
	if len(qs) > 0 {
		url += "?" + strings.Join(qs, "&")
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// ExportRow is one raw attendance record with relation names already resolved.
type ExportRow struct {
	ParticipantName string
	SiteName        string
	EventName       string
	Date            string
	Status          string
	RecordedByName  string
	CheckInTime     string
	Note            string
}

// handleExportCSV streams attendance records as a CSV download.
// It is wrapped with requireRole("admin").
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(hst)
	defTo := now.Format("2006-01-02")
	defFrom := now.AddDate(0, 0, -13).Format("2006-01-02")

	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	fromT, errFrom := time.Parse("2006-01-02", from)
	toT, errTo := time.Parse("2006-01-02", to)
	if errFrom != nil || errTo != nil {
		from, to = defFrom, defTo
		fromT, _ = time.Parse("2006-01-02", from)
		toT, _ = time.Parse("2006-01-02", to)
	}
	if fromT.After(toT) {
		fromT, toT = toT, fromT
		from, to = fromT.Format("2006-01-02"), toT.Format("2006-01-02")
	}
	if toT.Sub(fromT) > 30*24*time.Hour {
		toT = fromT.AddDate(0, 0, 29)
		to = toT.Format("2006-01-02")
	}

	eventID := strings.TrimSpace(r.URL.Query().Get("event"))
	if !requireEventID(w, eventID) {
		return
	}

	rows, err := s.loadExportRows(eventID, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	records := exportCSVRecords(rows)

	filename := "attendance_export_" + now.Format("2006-01-02") + ".csv"
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	cw := csv.NewWriter(w)
	for _, rec := range records {
		if err := cw.Write(rec); err != nil {
			http.Error(w, "csv write failed", http.StatusInternalServerError)
			return
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		http.Error(w, "csv flush failed", http.StatusInternalServerError)
		return
	}
}

// loadExportRows fetches raw attendance records matching the filters and
// resolves participant/site/event/recorder names for display.
func (s *Server) loadExportRows(eventID, from, to string) ([]ExportRow, error) {
	attCol, err := s.attendanceCollection()
	if err != nil {
		return nil, err
	}
	eventsCol, err := s.eventsCollection()
	if err != nil {
		return nil, err
	}
	eventIDs, err := s.resolveEventIDs(eventID)
	if err != nil {
		return nil, err
	}
	filter := fmt.Sprintf("date>='%s' && date<='%s'",
		mcpmod.EscapeFilter(from), mcpmod.EscapeFilter(to))
	if eventIDs != nil {
		ors := make([]string, 0, len(eventIDs))
		for _, id := range eventIDs {
			ors = append(ors, "event='"+mcpmod.EscapeFilter(id)+"'")
		}
		filter += " && (" + strings.Join(ors, " || ") + ")"
	}
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "date,intake", 10000, 0)
	if err != nil {
		return nil, err
	}

	out := make([]ExportRow, 0, len(recs))
	for _, rec := range recs {
		// Location is derived from the event's site, never from attendance.site.
		eventRec, err := s.pb.FindRecordById(eventsCol.Id, rec.GetString("event"))
		siteName := ""
		if err == nil {
			siteName = s.nameFor("sites", eventRec.GetString("site"))
		}
		out = append(out, ExportRow{
			ParticipantName: s.nameFor("intake", rec.GetString("intake")),
			SiteName:        siteName,
			EventName:       s.nameFor("events", rec.GetString("event")),
			Date:            rec.GetString("date"),
			Status:          rec.GetString("status"),
			RecordedByName:  s.nameFor("users", rec.GetString("recorded_by")),
			CheckInTime:     formatTime(rec.GetString("check_in_time")),
			Note:            rec.GetString("note"),
		})
	}
	return out, nil
}

// nameFor resolves a related record id to its display name. It returns ""
// for empty ids or failed lookups (e.g., cascade-deleted related records).
func (s *Server) nameFor(collection, id string) string {
	if id == "" {
		return ""
	}
	rec, err := s.pb.FindRecordById(collection, id)
	if err != nil {
		return ""
	}
	return rec.GetString("name")
}

// exportCSVRecords builds the full CSV table: header, one row per record,
// then a summary row. It is pure (no *Server/pb dependency) so unit tests
// can call it directly.
func exportCSVRecords(rows []ExportRow) [][]string {
	records := [][]string{
		{"Participant", "Site", "Event", "Date", "Status", "Recorded By", "Check-in Time", "Note"},
	}
	for _, r := range rows {
		records = append(records, []string{
			r.ParticipantName,
			r.SiteName,
			r.EventName,
			r.Date,
			exportStatus(r.Status),
			r.RecordedByName,
			r.CheckInTime,
			r.Note,
		})
	}
	records = append(records, summaryCSVRow(rows))
	return records
}

// exportStatus maps the stored status select value to title-case display.
func exportStatus(s string) string {
	switch s {
	case "present":
		return "Present"
	case "absent":
		return "Absent"
	case "excused":
		return "Excused"
	case "walk_in":
		return "Walk-in"
	}
	return ""
}

// summaryCSVRow returns the trailing summary row. The summary text is placed
// in the first column as a single human-readable string; remaining columns
// are empty. If the sibling test card t_7c6efa05 needs different wording,
// change only this function.
func summaryCSVRow(rows []ExportRow) []string {
	totalCheckIns := 0
	seen := map[string]bool{}
	presentCount := 0
	days := map[string]bool{}
	for _, r := range rows {
		if r.Status == "present" || r.Status == "walk_in" {
			totalCheckIns++
		}
		if r.Status == "present" {
			presentCount++
		}
		if r.ParticipantName != "" {
			seen[r.ParticipantName] = true
		}
		days[r.Date] = true
	}
	rate := 0
	if len(seen) > 0 && len(days) > 0 {
		rate = presentCount * 100 / (len(seen) * len(days))
	}
	return []string{
		fmt.Sprintf("Summary: %d check-ins, %d unique participants, %d%% avg rate",
			totalCheckIns, len(seen), rate),
		"", "", "", "", "", "", "",
	}
}
