# Working Plan: Implement CSV Attendance Export Handler and Filtering Logic

## Objective

Implement the CSV attendance export endpoint (FR14, UX-DR5) so that an admin can download the attendance matrix data for a chosen event, site, and date range as a properly-formatted CSV file with a summary stats row.

Add a `GET /attendance/export` route backed by a new `handleExportCSV` handler in `r3-intake/internal/server/attendance.go`, registered in `server.go` on branch `epic/3-csv-export`. The handler must:
- Honor `?event={id}&site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD` query filters with the same defaults as the matrix (last 14 days, user's assigned/first active site, no event filter).
- Emit correct HTTP headers (`Content-Type: text/csv`, `Content-Disposition` with `filename=attendance_export_YYYY-MM-DD.csv`).
- Produce CSV columns: **Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note** (PRD §05 Screen 2).
- Append a **summary row at the bottom** (total check-ins, unique participants, average attendance rate) per PRD §11 Acceptance and §05 Story 3.1 AC.
- Be factorable into a pure, testable function (the sibling test card `t_7c6efa05` will add unit/integration tests after this card).

## Constraints

- **Access control — DECISION:** Use `requireRole("admin", s.handleExportCSV)`. PRD §10 shows Export CSV as admin-only (✓ admin, ✗ case_manager) and this overrides the generic "auth" note in §09's route map. State this decision explicitly in code comments and register the route with `requireRole("admin", ...)`.
- **No `app.dao()`.** PocketBase v0.39 in-process API only: use `s.pb.FindRecordsByFilter(col.Id, filter, sort, limit, offset)` and `s.pb.FindRecordById(col.Id, id)`; escape all interpolated values with `mcpmod.EscapeFilter(...)`.
- **Date handling:** All dates are text `YYYY-MM-DD`. All "today" references use `time.Now().In(hst)` (`hst` is defined in `admin.go`). Default range = today minus 13 days → today (14 days inclusive). Cap range at 30 days from the start date (same as `handleMatrix`).
- **Existing helpers to reuse:** `s.resolveSite(u, param)`, `s.loadEvents(siteID)`, `s.currentSession(r)`, `mcpmod.EscapeFilter`, `buildDateRange`, `cycleStatus`, `formatTime` (HST display, `admin.go:926`). `s.pb.FindRecordById` is already used elsewhere for lookups.
- **Do not reuse `loadMatrixRows` for export** — it collapses statuses into per-day cells and drops `recorded_by`, `check_in_time`, and `note`. The export needs the raw attendance records with those fields.
- **Field mapping notes (schema 007):** `intake` relation → participant Name; `site` relation → site Name; `event` relation (nullable) → event Name or empty; `recorded_by` (nullable relation to users) → user Name or empty; `check_in_time` (text, nullable) → format via `formatTime` or empty; `note` (text, nullable, ≤500) → note or empty; `status` select: `present/absent/excused/walk_in` → title-case display (Present/Absent/Excused/Walk-in).
- **`encoding/csv`:** Use `csv.Writer`. Always `Flush()` and check `Error()` before writing the body. Escape quoting is handled automatically by the writer; do not hand-quote.
- **Method guard:** The route is registered with the `GET ` method pattern in the mux, so no explicit method check is required (Go 1.22+ patterns). Keep the handler signature `func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request)`.
- Branch `epic/3-csv-export` is the work branch.

## File Structure

```
r3-intake/internal/server/attendance.go   # ADD handleExportCSV + exportRows + helpers (below)
r3-intake/internal/server/server.go       # REGISTER route (one line)
r3-intake/internal/server/attendance_test.go  # optional: add unit test for exportCSVRecords (test card t_7c6efa05 will expand)
```

No new files required. All additions live in `attendance.go`; the route registration is a single line in `server.go` `Mux()`.

## Implementation Notes

### 1. Register the route in `server.go`

In `Mux()`, in the attendance block (currently lines ~132–135), add immediately after `mux.HandleFunc("/attendance", s.requireAuth(s.handleMatrix))`:

```go
// CSV export (FR14) — admin only (PRD §10: case_manager has no export access).
mux.HandleFunc("GET /attendance/export", s.requireRole("admin", s.handleExportCSV))
```

### 2. `handleExportCSV` — parse filters (mirror `handleMatrix` logic)

```go
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r) // guaranteed non-nil by requireRole

	// from/to with defaults (last 14 days) and 30-day cap — same as handleMatrix.
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
	siteID, _ := s.resolveSite(u, strings.TrimSpace(r.URL.Query().Get("site")))

	rows, err := s.loadExportRows(siteID, eventID, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	records := exportCSVRecords(rows) // pure, testable

	// Set headers BEFORE writing anything.
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
```

> Note: `Content-Disposition: attachment` is the correct choice so browsers download rather than render the file. Using `%q` around the filename handles any escaping the filename needs.

### 3. `loadExportRows` — fetch raw attendance records + resolve lookups

Query `attendance` directly (NOT via `loadMatrixRows`), filtering by date range, site, and event:

```go
// loadExportRows returns raw attendance records matching the filters, with
// participant/site/event/recorded-by names resolved for the export rows.
func (s *Server) loadExportRows(siteID, eventID, from, to string) ([]ExportRow, error) {
	attCol, err := s.attendanceCollection()
	if err != nil {
		return nil, err
	}
	filter := fmt.Sprintf("date>='%s' && date<='%s'", mcpmod.EscapeFilter(from), mcpmod.EscapeFilter(to))
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
```

**Name resolution helper** (batch-load each referenced collection once into a `map[id]name` to avoid N+1 `FindRecordById` calls; keep it simple):

```go
// nameFor resolves a related record id to its display name; returns "" when
// id is empty or the lookup fails.
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
```

> Performance note: For typical datasets this is fine. If desired, batch-load each referenced collection once (intake, sites, events, users) into `map[id]name` and look up from the maps instead of calling `FindRecordById` per row — prefer the batched approach if the sibling tests exercise large exports. Keep the helper signature pure/simple so tests can call it.
>
> **Pitfall:** `recorded_by` references the `users` collection (PocketBase auth collection). `FindRecordById("users", id)` works with the collection name; verify against the actual collection id/name used by the project (auth collection is typically named `users`).

### 4. `ExportRow` struct + pure CSV builder (testable)

```go
// ExportRow is one CSV data row, with names already resolved.
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
```

```go
// exportCSVRecords builds the full CSV table: header row, one row per
// attendance record, then a summary row. It is pure (no server/pb access) so
// unit tests can call it directly.
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
```

```go
// exportStatus maps the stored status select to a display string.
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
```

### 5. Summary row (PRD §11 "Summary row at bottom", §05 Story 3.1 AC)

```go
// summaryCSVRow returns the trailing summary row: total check-ins, unique
// participants, and average attendance rate over the range.
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
	// Average attendance rate: % of (unique participant × day) cells marked present.
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
```

**Design note:** The PRD is intentionally light on the exact summary wording ("Summary row at bottom" + Story 3.1 AC covering total, avg rate, dropout). The sibling test card (`t_7c6efa05`) will assert the exact expected summary. To keep that card stable, put the summary into a **single human-readable cell in the first column** (e.g. `Summary: 12 check-ins, 5 unique participants, 48% avg rate`) and document this choice in a code comment. When the test card lands, adjust wording in this one function only.

## Verification Criteria

From repo root `r3-intake/`:

1. **Build & vet & test must pass:**
   ```
   go build ./... && go vet ./... && go test ./...
   ```
   All three must exit 0 with no errors.

2. **Route registered:** `grep -n "attendance/export" internal/server/server.go` shows the `requireRole("admin", s.handleExportCSV)` registration.

3. **Admin-only guard:** Confirm non-admin (case_manager) session is rejected (403/redirect) and admin session is accepted — verify `requireRole` behavior in `auth.go:77` returns the intended response for a non-admin.

4. **Headers (manual curl with a session cookie):**
   ```
   curl -s -D - -o /dev/null "http://localhost:PORT/attendance/export?from=2026-08-01&to=2026-08-13"
   ```
   Must show:
   - `Content-Type: text/csv`
   - `Content-Disposition: attachment; filename="attendance_export_YYYY-MM-DD.csv"` (today's HST date)
   - HTTP 200.

5. **CSV shape:** Open/download the output in a spreadsheet app. Verify:
   - Header row exactly: `Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note`.
   - One data row per attendance record in the filtered range.
   - Empty event / recorded_by / note / check_in_time appear as empty cells (not `"<nil>"`).
   - Fields with commas/quotes/newlines are correctly quoted by `encoding/csv` (verify a note containing a comma).
   - A trailing **Summary** row is present with total check-ins, unique participant count, and average rate.
   - Dates are `YYYY-MM-DD`; check-in times are HST-formatted via `formatTime`.

6. **Filter correctness (manual + in tests):**
   - `?event={id}` → only that event's attendance.
   - `?site={id}` (admin) → only that site; blank site → all locations.
   - `?from`/`?to` → only records in range; malformed/blank → defaults to last 14 days; inverted range is swapped; >30-day range is capped to 30.
   - No params → last 14 days + admin's default (all locations if admin).

7. **Testability handoff:** `exportCSVRecords` (and `summaryCSVRow`) are pure functions with no `*Server`/`pb` dependency, so the sibling test card `t_7c6efa05` can unit-test row/header/summary formatting without a running server. Add at least one minimal unit test (e.g. `TestExportCSVRecords` asserting header + summary row) in `attendance_test.go` this card.
