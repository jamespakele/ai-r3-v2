package server

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
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
	// disable toggling and hide the walk-in panel.
	EventRequired bool
	// HasNoLocation reports whether at least one row is a no-location
	// participant, so the template can render the "No Location" group header.
	HasNoLocation bool
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
	NoLocation   bool // participant has no assigned site
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

	from, to, eventID, siteID, siteName, dates := s.parseMatrixFilters(r, u)

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

	hasNoLocation := false
	for _, row := range rows {
		if row.NoLocation {
			hasNoLocation = true
			break
		}
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
		EventRequired: eventID == "",
		HasNoLocation: hasNoLocation,
	}

	if r.Header.Get("HX-Request") == "true" {
		_ = s.tpl.ExecuteTemplate(w, "matrix-content", view)
		return
	}
	_ = s.tpl.ExecuteTemplate(w, "matrix", view)
}

// parseMatrixFilters derives the effective attendance filters from the
// request query and the session user, applying the same defaults and
// validation used by the matrix: a 14-day window (today and the prior 13
// days), inverted ranges swapped, and ranges capped at 30 days. The event
// filter is read from the query key "event" (matrix filter bar) and falls
// back to "event_id" (toggle/walk-in forms).
func (s *Server) parseMatrixFilters(r *http.Request, u *sessionUser) (from, to, eventID, siteID, siteName string, dates []string) {
	// Parse and validate from/to.
	now := time.Now().In(hst)
	defTo := now.Format("2006-01-02")
	defFrom := now.AddDate(0, 0, -13).Format("2006-01-02")

	from = strings.TrimSpace(r.URL.Query().Get("from"))
	to = strings.TrimSpace(r.URL.Query().Get("to"))
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

	eventID = strings.TrimSpace(r.URL.Query().Get("event"))
	if eventID == "" {
		eventID = strings.TrimSpace(r.URL.Query().Get("event_id"))
	}

	siteID, siteName = s.resolveSite(u, strings.TrimSpace(r.URL.Query().Get("site")))

	dates = buildDateRange(from, to)
	return from, to, eventID, siteID, siteName, dates
}

// handleStats renders only the stat-cards fragment for the current filters.
// It is wrapped with requireAuth in the mux and only ever called via HTMX
// after a dot toggle, so it shares parseMatrixFilters with handleMatrix to
// guarantee the cards always match the matrix.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	_, to, eventID, siteID, _, dates := s.parseMatrixFilters(r, u)

	rows, err := s.loadMatrixRows(u, siteID, dates, eventID, to)
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

	// Participants: ALWAYS the full site/role-scoped roster, independent of
	// the selected event (AC #1). The event only scopes the attendance map
	// below; it must never change which participants are listed.
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
		attFilter += fmt.Sprintf(" && (site='' || site='%s')", mcpmod.EscapeFilter(siteID))
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
		cellSiteID := siteID
		if cellSiteID == "" {
			cellSiteID = rec.GetString("site")
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
		row.IsDropout = row.LastPresent != "" && row.LastPresent < thresholdStr
		row.NoLocation = cellSiteID == ""
		rows = append(rows, row)
	}
	// Group no-location participants at the top, preserving the existing name
	// order within each group.
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].NoLocation && !rows[j].NoLocation
	})
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
	// The intake record is also the source of the effective site for
	// participants with no explicit site selected.
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
			siteID = rec.GetString("site")
		}
	}
	if siteID == "" && !intakeLoaded {
		if rec, err := s.pb.FindRecordById(intakeCol.Id, intakeID); err == nil {
			siteID = rec.GetString("site")
		}
	}

	// Attendance cannot be toggled for a participant with no assigned
	// location. The intake record is the source of truth for the site.
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
		rec.Set("event", eventID)
		_ = s.pb.Save(rec)
	case next != "" && existing != nil:
		existing.Set("status", next)
		existing.Set("recorded_by", u.ID)
		existing.Set("check_in_time", now)
		existing.Set("event", eventID)
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
	eventID := strings.TrimSpace(r.FormValue("event_id"))
	if !requireEventID(w, eventID) {
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

	// Idempotent upsert for (event, intake, date).
	filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
		mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(today))
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().In(hst).Format("2006-01-02 15:04:05")
	if len(recs) > 0 {
		existing := recs[0]
		existing.Set("status", "walk_in")
		existing.Set("site", siteID)
		existing.Set("event", eventID)
		existing.Set("recorded_by", u.ID)
		existing.Set("check_in_time", now)
		_ = s.pb.Save(existing)
	} else {
		rec := core.NewRecord(attCol)
		rec.Set("intake", intakeID)
		rec.Set("site", siteID)
		rec.Set("event", eventID)
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
// It is wrapped with requireRole("admin"), so u is non-nil here.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)

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
	siteID, _ := s.resolveSite(u, strings.TrimSpace(r.URL.Query().Get("site")))

	rows, err := s.loadExportRows(siteID, eventID, from, to)
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
func (s *Server) loadExportRows(siteID, eventID, from, to string) ([]ExportRow, error) {
	attCol, err := s.attendanceCollection()
	if err != nil {
		return nil, err
	}
	filter := fmt.Sprintf("date>='%s' && date<='%s'",
		mcpmod.EscapeFilter(from), mcpmod.EscapeFilter(to))
	if siteID != "" {
		filter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(siteID))
	}
	if eventID != "" {
		filter += fmt.Sprintf(" && event='%s'", mcpmod.EscapeFilter(eventID))
	}
	recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "date,intake", 10000, 0)
	if err != nil {
		return nil, err
	}

	out := make([]ExportRow, 0, len(recs))
	for _, rec := range recs {
		out = append(out, ExportRow{
			ParticipantName: s.nameFor("intake", rec.GetString("intake")),
			SiteName:        s.nameFor("sites", rec.GetString("site")),
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
