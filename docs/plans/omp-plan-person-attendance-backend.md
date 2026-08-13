# Working Plan: Participant Attendance History & Edits — Backend (Story 4, Epic 4)

## Objective

Implement the **backend** for per-participant attendance history and edits in the
R3 Intake Go server. This card delivers the Go handler functions, view data
structs, routes, data-loading logic (monthly calendar grid, per-person stats,
current streak), and the day-detail save/delete handlers. It defines the view
structs and template names that the sibling UI cards (`t_cca37bee` calendar UI,
`t_1030de8b` day-detail/edit UI) will consume, and includes **minimal template
stubs** only so the page renders and tests pass. The actual HTML/CSS polish is
owned by the sibling UI cards.

Functional requirements covered (backend portion):
- **FR15** — `GET /intake/{id}/attendance` renders a monthly calendar of a single
  participant's attendance history, defaulting to the current month, with
  `?month=YYYY-MM` navigation.
- **FR16** — per-person stats: participant name, site name, "X of Y days (Z%)"
  (X = present+walk_in count, Y = total attendance records in visible month,
  Z = rate %, color-coded green >=50% / red <50%), plus "Current streak"
  (consecutive present days ending today or most recent present date). Empty
  state: "0 of 0 days (0%)" and "No attendance recorded yet".
- **FR17** — day detail & edit: modal with status dropdown, event name (if
  linked), recorded-by name, check-in time, editable note, Save/Cancel; empty
  days show "No attendance recorded" with option to add; delete with
  confirmation.
- **FR20** — attendance tab reachable from the intake record (a link/route; the
  tab chrome itself is UI-card work, but the route must exist and be reachable).

Backend endpoints to expose:
1. `GET /intake/{id}/attendance` — full page render (monthly calendar + stats).
   Auth: `requireAuth`. Case managers may only view their own assigned intakes
   (`assigned_to == u.ID`); admins any.
2. `GET /intake/{id}/attendance/day?date=YYYY-MM-DD` — day-detail fragment
   (modal content) for a specific date: existing record (status, event name,
   recorded-by name, check-in time, note) or empty state.
3. `POST /intake/{id}/attendance/day` — save/update an attendance record for a
   date (status dropdown + note). Creates new if none exists, updates existing.
   Fields: `date`, `status`, `note`. `site` derived from the intake record;
   `recorded_by` = current user.
4. `POST /intake/{id}/attendance/day/delete` — delete an attendance record for
   a date.

## Constraints

- **Language/runtime:** Go (module `r3-intake`), server-rendered Go templates,
  HTMX + Alpine.js, vanilla CSS. No new dependencies.
- **PocketBase v0.39 JS API** via `app.findCollectionByNameOrId` (migrations) and
  Go `s.pb.*` (policy layer). Browser never talks to PB directly; all collection
  rules are `null`. Go is the policy layer.
- **No native unique constraints in PB** — enforce `(intake, date)` idempotency
  in Go by querying for an existing record before create.
- **Filter escaping:** any user-supplied value interpolated into a PB filter
  string MUST go through `mcpmod.EscapeFilter(...)` (import
  `mcpmod "r3-intake/internal/mcp"`).
- **HST timezone:** `var hst = time.FixedZone("HST", -10*60*60)` (in admin.go).
  "Current month" and "today" derive from `time.Now().In(hst)`. Dates are text
  `YYYY-MM-DD`; timestamps `"2006-01-02 15:04:05"`.
- **Existing patterns:** follow `notes.go` (per-participant page: parse
  `/notes/{intakeID}/...`, `findIntake`, build view struct, `ExecuteTemplate`),
  `handleToggle` (authz + idempotent attendance upsert), and the matrix
  full-page + fragment split (`matrix` / `matrix-content`).
- **Route ordering:** Go ServeMux matches longest prefix. `/intake/` is already
  registered to `handleIntakeCmd`; register the more specific
  `/intake/{id}/attendance` patterns so they win over the catch-all.
- **Templates:** single embedded `internal/assets/public/index.html`; new
  `{{define}}` blocks go at the END of the file. Reference via
  `s.tpl.ExecuteTemplate(w, "name", view)`.
- **Tests:** white-box (`package server`). Unit tests parse the embedded
  template via `assets.TemplateString()` + `templateFuncs()`. Integration tests
  boot a real in-process PocketBase via `newTestServer(t)` and seed via
  `seedExportData` (reuse/extend the existing harness in
  `attendance_export_integration_test.go`).

## File Structure

### Create
- `r3-intake/internal/server/person_attendance.go` — all new handlers, view
  structs, and pure helpers for the per-person attendance page. (Keeps
  `attendance.go` focused on the matrix/export; mirrors how `notes.go` is a
  separate per-participant file.)
- `r3-intake/internal/server/person_attendance_test.go` — unit tests for pure
  helpers (month grid, stats, streak) and template render.
- `r3-intake/internal/server/person_attendance_integration_test.go` —
  route-level integration tests (authz, day save/update/delete, month render)
  using `newTestServer(t)` + a new `seedPersonAttendanceData` helper.

### Modify
- `r3-intake/internal/server/server.go` — register the new routes in `Mux()`.
- `r3-intake/internal/assets/public/index.html` — append minimal template stubs:
  `person-attendance` (full page), `person-attendance-calendar` (calendar +
  stats fragment), `person-attendance-day` (day-detail modal fragment). Stubs
  render the view struct fields so tests pass; visual polish is the sibling UI
  cards' job.

## Implementation Notes

### Routes (in `Mux()`, after the existing `/intake/` catch-all)
```go
mux.HandleFunc("/intake/{id}/attendance", s.requireAuth(s.handlePersonAttendance))
mux.HandleFunc("/intake/{id}/attendance/day", s.requireAuth(s.handlePersonAttendanceDay))
mux.HandleFunc("/intake/{id}/attendance/day/delete", s.requireAuth(s.handlePersonAttendanceDayDelete))
```
Go's longest-prefix matching means `/intake/{id}/attendance` and its subpaths
outrank the `/intake/` catch-all. Parse `{id}` from `r.PathValue("id")` (Go 1.22+
pattern) or via `strings.TrimPrefix` like `handleNotes` — prefer
`r.PathValue("id")` since the pattern uses `{id}`.

### Shared intake-loading + authz helper
Add one helper used by all three handlers (mirrors `handleToggle` authz):
```go
func (s *Server) loadPersonAttendanceIntake(w http.ResponseWriter, u *sessionUser, id string) (*core.Record, bool)
```
- `findIntake(id)`; on error → `http.NotFound`.
- If `u.Role == "case_manager"` and `rec.GetString("assigned_to") != u.ID` →
  `http.Error(w, "forbidden", http.StatusForbidden)` and return `ok=false`.
- Admins pass any intake.
- Returns the decrypted intake record (already decrypted by `findIntake`).

### View data structs (consumed by sibling UI cards)
```go
type PersonAttendanceView struct {
    UserName   string
    Role       string
    IsAdmin    bool
    IntakeID   string
    IntakeName string
    SiteName   string
    Month      string // "YYYY-MM" of the visible month
    PrevMonth  string // "YYYY-MM"
    NextMonth  string // "YYYY-MM"
    Today      string // "YYYY-MM-DD" (HST)
    Weekdays   []string // ["Sun","Mon",...] 7 columns
    Weeks      [][]PersonDayCell // grid rows; each row = 7 cells
    Stats      PersonStats
    Legend     []PersonLegendItem
}

type PersonDayCell struct {
    Date       string // "YYYY-MM-DD"
    DayNum     string // "1".."31"
    Status     string // "" | "present" | "absent" | "excused" | "walk_in"
    StatusLabel string // "" | "Present" | ... (via exportStatus)
    IsToday    bool
    IsOtherMonth bool // leading/trailing blank cells from adjacent months
    HasRecord  bool
}

type PersonStats struct {
    PresentCount int // status in {present, walk_in}
    TotalDays    int // total attendance records in visible month
    Rate         int // percent 0-100 (0 when TotalDays==0)
    RateColor    string // "green" | "red" (green >=50, red <50)
    Streak       int // consecutive present days ending today or most recent present
    HasRecords   bool
}

type PersonLegendItem struct {
    Status string
    Label  string
    Color  string // css class/hint token
}
```
`PersonDayCell.StatusLabel` reuses `exportStatus` from `attendance.go` for
consistent title-casing. `PersonStats.RateColor` is computed in Go so the
template only branches on `eq .RateColor "green"`.

### `handlePersonAttendance` (GET full page)
1. `u := s.currentSession(r)`; `intake, ok := s.loadPersonAttendanceIntake(...)`.
2. Resolve month: `?month=YYYY-MM` query param; validate with
   `time.Parse("2006-01", month)`; default to `time.Now().In(hst).Format("2006-01")`.
3. Compute month bounds: first day of month, last day of month (use
   `time.Date(y, m+1, 0, ...)` trick for last day), plus leading/trailing
   padding to align to a 7-column grid starting Sunday.
4. Load attendance records for the intake within `[firstDay, lastDay]`:
   ```go
   filter := fmt.Sprintf("intake='%s' && date>='%s' && date<='%s'",
       mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(firstDay), mcpmod.EscapeFilter(lastDay))
   recs, err := s.pb.FindRecordsByFilter(attCol.Id, filter, "date", 1000, 0)
   ```
   Build a `map[date]status` for O(1) cell lookup.
5. Build the grid: iterate day-by-day from the padded start to padded end,
   producing `Weeks [][]PersonDayCell`. Mark `IsToday` when date == HST today.
6. Compute `PersonStats` from the loaded records (see below).
7. Resolve site name: `s.nameFor("sites", intake.GetString("site"))` (reuse
   `nameFor` helper).
8. `s.tpl.ExecuteTemplate(w, "person-attendance", view)`.

### Stats computation (pure helper, unit-testable)
```go
func computePersonStats(records []PersonAttendanceRecord, today string) PersonStats
```
- `PresentCount` = count of records with `status` in `{present, walk_in}`.
- `TotalDays` = `len(records)`.
- `Rate` = `PresentCount * 100 / TotalDays`, guarding `TotalDays == 0` → 0.
- `RateColor` = `"green"` if `Rate >= 50` else `"red"`.
- `Streak`: sort present/walk_in dates ascending; walk backward from the most
  recent present date. If the most recent present date is not `today` and not
  `today-1`... define streak as consecutive present days ending at the most
  recent present date (per FR16: "ending today (or most recent present date)").
  Count consecutive calendar days that have a present/walk_in record, stopping
  at the first gap. If no present records → streak 0.
- `HasRecords` = `TotalDays > 0` (drives the empty-state text).

### `handlePersonAttendanceDay` (GET day fragment)
1. Authz via `loadPersonAttendanceIntake`.
2. `date := r.URL.Query().Get("date")`; validate `time.Parse("2006-01-02", date)`.
3. Query existing record: `intake='...' && date='...'` (limit 1). If found,
   build a `PersonDayDetailView` with status, event name
   (`s.nameFor("events", rec.GetString("event"))`), recorded-by name
   (`s.nameFor("users", rec.GetString("recorded_by"))`), `check_in_time`, `note`.
   If not found, build the empty-state variant (`HasRecord=false`).
4. `s.tpl.ExecuteTemplate(w, "person-attendance-day", view)` — returns the modal
   fragment (HTMX target). The sibling UI card owns the modal chrome; this card
   provides the data + fragment template stub.

### `handlePersonAttendanceDay` (POST save/update)
1. Authz via `loadPersonAttendanceIntake`.
2. Read form fields: `date`, `status`, `note`. Validate:
   - `date` parses as `2006-01-02`.
   - `status` is one of `{present, absent, excused, walk_in}` (reject otherwise).
   - `note` trimmed, max 500 chars (truncate or reject; prefer reject with 400).
3. Query existing `(intake, date)` record (limit 1).
   - If exists → update: set `status`, `note`; keep `site`, `event`,
     `check_in_time`, `recorded_by` as-is (or update `recorded_by` to current
     user per spec — see note below). `s.pb.Save(rec)`.
   - If not exists → create `core.NewRecord(attCol)`: set `intake`, `date`,
     `status`, `note`, `site` = `intake.GetString("site")`, `recorded_by` =
     `u.ID`. `s.pb.Save(rec)`.
4. On success, respond with the re-rendered calendar fragment
   (`person-attendance-calendar`) so the HTMX swap refreshes the grid + stats
   without a full page reload. On validation error, return the day fragment with
   an error message (or a 400 with a plain message — keep simple; sibling UI
   card wires the UX).

> **Design decision — `recorded_by` on update:** spec says "recorded_by = current
> user" for the save endpoint. For an update, set `recorded_by` to the current
> user (the person who last edited). This is the simplest reading and matches
> "recorded by (user name)" shown in the modal. If the sibling UI card needs
> original-author preservation, revisit — but default to current user.

### `handlePersonAttendanceDayDelete` (POST delete)
1. Authz via `loadPersonAttendanceIntake`.
2. `date` from form; validate.
3. Query existing `(intake, date)` record; if found, `s.pb.Delete(rec)`.
   If not found, treat as no-op success (idempotent).
4. Respond with the re-rendered calendar fragment.

### Unique constraint on `(intake, date)`
No native PB unique constraint. Enforce in Go: always query
`intake='...' && date='...'` (limit 1) before create; if a record exists, update
it instead of creating a duplicate. This is the same idempotency pattern used by
`handleToggle` and the enrollment junction. The `date` field is text `YYYY-MM-DD`
so the filter is exact.

### Edge cases
- **Empty month:** no records → `PersonStats{0,0,0,"red",0,false}`; template
  shows "0 of 0 days (0%)" and "No attendance recorded yet".
- **Invalid `?month`:** fall back to current month (HST).
- **Invalid `?date` on day fragment:** 400.
- **Case manager on another CM's intake:** 403 (authz helper).
- **Intake not found:** 404.
- **Month boundary padding:** leading/trailing cells from adjacent months render
  as blank/`IsOtherMonth` cells so the grid is always a full 7-column rectangle.
- **Streak with no present records:** 0.
- **Rate division by zero:** guard `TotalDays == 0` → rate 0, color red.
- **Note length:** cap at 500 (matches migration `max: 500`).

### Template stubs (minimal, in `index.html`)
Append at end of file:
- `{{define "person-attendance"}}` — full `<!DOCTYPE html>` page shell (topbar
  like `notes`/`matrix`), heading with participant + site name, then
  `{{template "person-attendance-calendar" .}}`.
- `{{define "person-attendance-calendar"}}` — stats row (`.Stats`), month
  navigation (`.PrevMonth`/`.NextMonth` links with `?month=`), weekday header
  (`.Weekdays`), grid (`.Weeks`), legend (`.Legend`). This is the HTMX swap
  target for POST responses.
- `{{define "person-attendance-day"}}` — day-detail modal fragment: status
  dropdown, event name, recorded-by, check-in time, note textarea, Save/Cancel,
  Delete (with `onsubmit="return confirm(...)"`), or empty-state "No attendance
  recorded" + add form.

The sibling UI cards (`t_cca37bee`, `t_1030de8b`) will flesh out these templates
and the CSS. This card only needs them to render the view structs without error
so unit/integration tests can assert on output.

### Status display mapping (reuse `exportStatus`)
`present→Present`, `absent→Absent`, `excused→Excused`, `walk_in→Walk-in`.
Colors (for the legend / cell classes, defined in CSS by UI card): present green
`#3f6b34`, absent gray `#eee`+border, excused yellow `#8a6a1e`, walk_in blue
`#2a4d8a`; present/walk_in cells green border `#cfe0c6` + light green bg
`#f6fbf4`; today accent border `#b5502e` 2px. This card exposes the status
values/labels in the view struct; the UI card applies the classes.

## Verification Criteria

### Build & static checks
```bash
cd r3-intake; go build ./...; go vet ./...; go test ./...
```
All must pass with no new failures.

### Unit tests (`person_attendance_test.go`)
- `TestComputePersonStats` — present+walk_in counting, rate rounding, division
  by zero (empty → 0/0/0), rate color boundary (49 red, 50 green, 51 green).
- `TestComputeStreak` — consecutive present days ending today; ending at most
  recent present date when today has no record; gap breaks streak; no present →
  0.
- `TestBuildMonthGrid` — correct number of weeks, 7 columns each, first/last day
  placement, `IsToday` marking, `IsOtherMonth` padding.
- `TestPersonAttendanceRender` — parse embedded template via
  `assets.TemplateString()` + `templateFuncs()`, `ExecuteTemplate` on
  `person-attendance`, `person-attendance-calendar`, and `person-attendance-day`
  with a populated view; assert on substrings (participant name, site name,
  "X of Y days (Z%)", streak, status labels, empty-state text).

### Integration tests (`person_attendance_integration_test.go`)
Reuse `newTestServer(t)`; add `seedPersonAttendanceData(t, pb)` (or extend
`seedExportData`) creating: site, event, admin, cm1, cm2, intake `i1` assigned to
`cm1`, intake `i2` assigned to `cm2`, and attendance records across a known
month (present, walk_in, absent, excused, plus a gap for streak testing).
Drive via `srv.Mux().ServeHTTP(rec, req)` with cookies from
`srv.makeSession(&sessionUser{...})`:
- `TestPersonAttendanceAuthz` — admin can view `i1` and `i2`; `cm1` can view
  `i1` (assigned) but gets 403 on `i2`; unauthenticated gets redirected/401.
- `TestPersonAttendanceMonthRender` — GET `/intake/{i1}/attendance?month=YYYY-MM`
  returns 200 and contains participant name, site name, expected stats text,
  and status labels.
- `TestPersonAttendanceDayGet` — GET day fragment for a date with a record
  returns status/event/recorded-by/note; for a date with no record returns the
  empty-state text.
- `TestPersonAttendanceDaySaveCreate` — POST creates a new record; subsequent
  GET day shows it; `site` derived from intake, `recorded_by` = current user.
- `TestPersonAttendanceDaySaveUpdate` — POST twice on same date updates the
  existing record (no duplicate; assert only one record for `(intake,date)`).
- `TestPersonAttendanceDayDelete` — POST delete removes the record; second delete
  is a no-op success.
- `TestPersonAttendanceDayValidation` — invalid date / invalid status → 400.

### Acceptance mapping
- FR15: month render + `?month=` navigation (unit grid + integration render).
- FR16: stats row + streak + empty state (unit stats/streak + render).
- FR17: day fragment GET + save/update/delete POST (integration).
- FR20: route reachable from `/intake/{id}` (route registration + integration
  render).
