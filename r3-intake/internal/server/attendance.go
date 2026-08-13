package server

import (
	"fmt"
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
}

// MatrixRow is one participant row in the matrix.
type MatrixRow struct {
	IntakeID     string
	Name         string
	Cells        []MatrixCell // one per date, in Dates order
	TotalDays    int
	PresentCount int
	WalkInCount  int
	LastPresent  string // YYYY-MM-DD or ""
	IsDropout    bool
}

// MatrixSummary holds the aggregate stat cards below the matrix, computed
// from the same rows that rendered the grid so cards always match the matrix.
type MatrixSummary struct {
	TotalCheckIns      int
	ActiveParticipants int
	Stopped            int
	AvgRate            int // 0–100, truncated (integer division)
}

// MatrixCell is a single date cell for a participant.
type MatrixCell struct {
	IntakeID string
	Date     string
	Status   string // "", "present", "absent", "excused", "walk_in"
	SiteID   string
	EventID  string
	From     string
	To       string
}

// attendanceCollection returns the attendance collection.
func (s *Server) attendanceCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("attendance")
}

// handleMatrix renders the attendance matrix screen. It is wrapped with
// requireAuth in the mux, so an authenticated session is guaranteed here.
func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)

	// Parse and validate from/to.
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
	// Cap range at 30 days from the start date.
	if toT.Sub(fromT) > 30*24*time.Hour {
		toT = fromT.AddDate(0, 0, 29)
		to = toT.Format("2006-01-02")
	}

	eventID := strings.TrimSpace(r.URL.Query().Get("event"))

	siteID, siteName := s.resolveSite(u, strings.TrimSpace(r.URL.Query().Get("site")))

	dates := buildDateRange(from, to)

	rows, err := s.loadMatrixRows(u, siteID, dates, eventID, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sites, err := s.loadSites(false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := s.loadEvents(siteID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := MatrixViewData{
		UserName: u.Name,
		Role:     u.Role,
		IsAdmin:  u.Role == "admin",
		SiteID:   siteID,
		SiteName: siteName,
		DateFrom: from,
		DateTo:   to,
		Dates:    dates,
		Rows:     rows,
		Sites:    sites,
		Events:   events,
		Summary:  computeSummary(rows, len(dates)),
		EventID:  eventID,
	}

	if r.Header.Get("HX-Request") == "true" {
		_ = s.tpl.ExecuteTemplate(w, "matrix-content", view)
		return
	}
	_ = s.tpl.ExecuteTemplate(w, "matrix", view)
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
	// case_manager: derive site from assigned intakes.
	counts := map[string]int{}
	col, err := s.intakeCollection()
	if err == nil {
		filter := fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
		recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "name", 1000, 0)
		if err == nil {
			for _, rec := range recs {
				sid := rec.GetString("site")
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
func (s *Server) loadMatrixRows(u *sessionUser, siteID string, dates []string, eventID, to string) ([]MatrixRow, error) {
	intakeCol, err := s.intakeCollection()
	if err != nil {
		return nil, err
	}
	attCol, err := s.attendanceCollection()
	if err != nil {
		return nil, err
	}

	// Participants.
	var intakeFilter string
	switch {
	case u.Role == "case_manager":
		intakeFilter = fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
	case siteID != "":
		intakeFilter = fmt.Sprintf("site='%s'", mcpmod.EscapeFilter(siteID))
	default:
		intakeFilter = "1=1"
	}
	intakeRecs, err := s.pb.FindRecordsByFilter(intakeCol.Id, intakeFilter, "name", 1000, 0)
	if err != nil {
		return nil, err
	}

	// Attendance map: intakeID -> date -> status.
	from := dates[0]
	toDate := dates[len(dates)-1]
	attFilter := fmt.Sprintf("date>='%s' && date<='%s'", mcpmod.EscapeFilter(from), mcpmod.EscapeFilter(toDate))
	if siteID != "" {
		attFilter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(siteID))
	}
	if eventID != "" {
		attFilter += fmt.Sprintf(" && event='%s'", mcpmod.EscapeFilter(eventID))
	}
	attRecs, err := s.pb.FindRecordsByFilter(attCol.Id, attFilter, "date", 5000, 0)
	if err != nil {
		return nil, err
	}
	attMap := map[string]map[string]string{}
	for _, rec := range attRecs {
		iid := rec.GetString("intake")
		d := rec.GetString("date")
		if attMap[iid] == nil {
			attMap[iid] = map[string]string{}
		}
		attMap[iid][d] = rec.GetString("status")
	}

	threshold, _ := time.Parse("2006-01-02", to)
	threshold = threshold.AddDate(0, 0, -13)
	thresholdStr := threshold.Format("2006-01-02")

	rows := make([]MatrixRow, 0, len(intakeRecs))
	for _, rec := range intakeRecs {
		iid := rec.Id
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
				SiteID:   siteID,
				EventID:  eventID,
				From:     from,
				To:       toDate,
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
		row.IsDropout = row.LastPresent != "" && row.LastPresent < thresholdStr
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
		if row.IsDropout {
			s.Stopped++
		}
		totalPresent += row.PresentCount
	}
	s.AvgRate = totalPresent * 100 / (len(rows) * days)
	return s
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

// loadEvents returns active events, optionally scoped to a single site. When
// siteID is "", all active events are returned (admin view).
func (s *Server) loadEvents(siteID string) ([]Event, error) {
	col, err := s.eventsCollection()
	if err != nil {
		return nil, err
	}
	filter := "status='active'"
	if siteID != "" {
		filter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(siteID))
	}
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

// cycleStatus returns the next status in the cycle
// "" -> present -> absent -> excused -> walk_in -> "".
func cycleStatus(current string) string {
	order := []string{"present", "absent", "excused", "walk_in"}
	if current == "" {
		return order[0]
	}
	for i, st := range order {
		if st == current {
			if i+1 < len(order) {
				return order[i+1]
			}
			return ""
		}
	}
	return order[0]
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

	if intakeID == "" || date == "" || siteID == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
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
	if u.Role == "case_manager" {
		intakeCol, err := s.intakeCollection()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rec, err := s.pb.FindRecordById(intakeCol.Id, intakeID)
		if err != nil || rec.GetString("assigned_to") != u.ID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	attCol, err := s.attendanceCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find existing record.
	filter := fmt.Sprintf("intake='%s' && date='%s'", mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(date))
	if eventID != "" {
		filter += fmt.Sprintf(" && event='%s'", mcpmod.EscapeFilter(eventID))
	}
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
	now := time.Now().In(hst).Format("2006-01-02 15:04:05.000Z")

	switch {
	case next == "" && existing != nil:
		_ = s.pb.Delete(existing)
	case next != "" && existing == nil:
		rec := core.NewRecord(attCol)
		rec.Set("intake", intakeID)
		rec.Set("site", siteID)
		rec.Set("date", date)
		rec.Set("status", next)
		rec.Set("recorded_by", u.ID)
		rec.Set("check_in_time", now)
		if eventID != "" {
			rec.Set("event", eventID)
		}
		_ = s.pb.Save(rec)
	case next != "" && existing != nil:
		existing.Set("status", next)
		existing.Set("recorded_by", u.ID)
		existing.Set("check_in_time", now)
		_ = s.pb.Save(existing)
	}

	cell := MatrixCell{
		IntakeID: intakeID,
		Date:     date,
		Status:   next,
		SiteID:   siteID,
		EventID:  eventID,
		From:     from,
		To:       to,
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

// walkinResult is one search result row for the matrix "Add walk-in" panel.
type walkinResult struct {
	ID      string
	Name    string
	SiteID  string
	From    string
	To      string
	EventID string
}

// handleWalkinSearch returns an HTML fragment listing intake records whose
// name matches the ?name= query, scoped to the resolved site (auth-only).
// Min 2 chars, max 10 results. Used by the matrix "Add walk-in" panel.
func (s *Server) handleWalkinSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("name"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(q) < 2 {
		return // empty body — no results for short queries
	}
	u := s.currentSession(r)
	col, err := s.intakeCollection()
	if err != nil {
		return
	}
	filter := fmt.Sprintf(`name ~ "%s"`, mcpmod.EscapeFilter(q))
	siteID, _ := s.resolveSite(u, strings.TrimSpace(r.URL.Query().Get("site_id")))
	if siteID != "" {
		filter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(siteID))
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 10, 0)
	if err != nil {
		return
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	eventID := strings.TrimSpace(r.URL.Query().Get("event_id"))
	results := make([]walkinResult, 0, len(recs))
	for _, rec := range recs {
		name := rec.GetString("name")
		if name == "" {
			name = "(unnamed)"
		}
		results = append(results, walkinResult{
			ID:      rec.Id,
			Name:    name,
			SiteID:  siteID,
			From:    from,
			To:      to,
			EventID: eventID,
		})
	}
	_ = s.tpl.ExecuteTemplate(w, "walkin-results", results)
}

// handleWalkin records a walk-in attendance cell for today at the resolved
// site. It accepts either an existing intake_id or a name to create a minimal
// intake, and is idempotent per (intake, date). POST only.
func (s *Server) handleWalkin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := s.currentSession(r)

	siteID, _ := s.resolveSite(u, strings.TrimSpace(r.FormValue("site_id")))
	if siteID == "" {
		http.Error(w, "no site resolved", http.StatusBadRequest)
		return
	}
	today := time.Now().In(hst).Format("2006-01-02")

	intakeCol, err := s.intakeCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Resolve the intake record.
	intakeID := strings.TrimSpace(r.FormValue("intake_id"))
	if intakeID == "" {
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		rec := core.NewRecord(intakeCol)
		rec.Set("name", name)
		rec.Set("site", siteID)
		rec.Set("created_by", u.ID)
		rec.Set("status", "unassigned")
		if err := s.pb.Save(rec); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		intakeID = rec.Id
	} else {
		if _, err := s.pb.FindRecordById(intakeCol.Id, intakeID); err != nil {
			http.Error(w, "intake not found", http.StatusNotFound)
			return
		}
	}

	attCol, err := s.attendanceCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Idempotent upsert for (intake, date).
	filter := fmt.Sprintf("intake='%s' && date='%s'", mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(today))
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().In(hst).Format("2006-01-02 15:04:05.000Z")
	if len(recs) > 0 {
		existing := recs[0]
		existing.Set("status", "walk_in")
		existing.Set("site", siteID)
		existing.Set("recorded_by", u.ID)
		existing.Set("check_in_time", now)
		_ = s.pb.Save(existing)
	} else {
		rec := core.NewRecord(attCol)
		rec.Set("intake", intakeID)
		rec.Set("site", siteID)
		rec.Set("date", today)
		rec.Set("status", "walk_in")
		rec.Set("recorded_by", u.ID)
		rec.Set("check_in_time", now)
		_ = s.pb.Save(rec)
	}

	// 303 redirect back to the matrix with the same filters.
	var qs []string
	if siteID != "" {
		qs = append(qs, "site="+siteID)
	}
	if from := strings.TrimSpace(r.FormValue("from")); from != "" {
		qs = append(qs, "from="+from)
	}
	if to := strings.TrimSpace(r.FormValue("to")); to != "" {
		qs = append(qs, "to="+to)
	}
	if eventID := strings.TrimSpace(r.FormValue("event_id")); eventID != "" {
		qs = append(qs, "event="+eventID)
	}
	url := "/attendance"
	if len(qs) > 0 {
		url += "?" + strings.Join(qs, "&")
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}
