# Working Plan: Calendar Matrix View Grid + HTMX Cell Toggle

## Objective

Implement the attendance **Calendar Matrix View** (Story 1.2) and the **HTMX cell toggle with auto-save** (Story 1.3) in the R3 Intake Go/PocketBase app. The matrix renders a spreadsheet grid — participants as rows, dates as columns, color-coded status dots — with a filter bar (site, from/to date), a sticky participant-name column, a per-participant "Total" present-count badge, and dropout highlighting. Clicking a cell cycles its status (empty → present → absent → excused → walk_in → empty) via an HTMX POST to `/attendance/toggle` that returns the updated cell fragment for swap, with a no-JS fallback that does a native POST → 303 redirect.

## Constraints

- **Go is the policy layer.** All PocketBase data access goes through `s.pb` in-process; the browser never talks to PB directly. All attendance/events collection rules are `null` (locked) — enforcement happens in Go.
- **No user→site field exists.** A `case_manager`'s site is derived from intakes where `assigned_to = <their user id>` (most common site); fallback to first active site. A `case_manager` cannot change site in the filter and sees only their site's participants. An `admin` can pick any active site or "All locations" (`site=""`).
- **Unique constraint enforced in Go.** Before create/update, check for an existing attendance record on `(intake, date)` or `(event, intake, date)` — never duplicate.
- **Date range capped at 30 days.** If `from`/`to` span more than 30 days, truncate to 30 days from the start date.
- **Default view:** last 14 days (HST), user's assigned site (or first active site for admin), no event filter.
- **Time zone:** use the existing `var hst = time.FixedZone("HST", -10*60*60)` in `r3-intake/internal/server/admin.go`; all date math and `check_in_time` use `time.Now().In(hst)`.
- **Filter escaping:** use `mcpmod.EscapeFilter(s)` from `r3-intake/internal/mcp/mcp.go` (import as `mcpmod "r3-intake/internal/mcp"`) for any user-supplied value interpolated into a PB filter string.
- **Templates:** single embedded `index.html` with multiple `{{define}}` blocks; new blocks go at the end. Reference via `s.tpl.ExecuteTemplate(w, "name", view)`.
- **HTMX:** use `hx-post` + `hx-target`/`hx-swap` for partial swaps; handlers return raw HTML fragments.
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must pass; template parse must succeed (all defines registered).

## File Structure

All paths relative to repo root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_1ad219e4`.

| File | Change |
|------|--------|
| `r3-intake/internal/server/attendance.go` | **Replace/extend.** Add `MatrixViewData`, `MatrixRow`, `MatrixCell` structs; replace `handleMatrix` body; add `handleToggle`, `cycleStatus`, `buildDateRange`, `resolveSiteID`, `loadMatrixRows`, `loadAttendanceMap`, `renderMatrixCell`, `matrixRedirectURL` helpers. |
| `r3-intake/internal/server/server.go` | **Add route** `mux.HandleFunc("/attendance/toggle", s.requireAuth(s.handleToggle))` next to the existing `/attendance` registration. |
| `r3-intake/internal/assets/public/index.html` | **Replace** the body of `{{define "matrix"}}` (currently a placeholder at line ~770). **Add** `{{define "matrix-cell"}}` partial at the end of the file for HTMX swap. |
| `r3-intake/internal/assets/public/app.css` | **Add** matrix-specific CSS classes (see Implementation Notes). Bump the `?v=` cache-buster on the stylesheet link in the matrix template. |
| `r3-intake/pocketbase/migrations/007_events_attendance.js` | **No change** (collections already exist). Reference only. |

## Implementation Notes

### 1. Data model — `r3-intake/internal/server/attendance.go`

Replace the current `AttendanceView` struct with a richer `MatrixViewData` (keep the existing field names so the topbar template keeps working):

```go
type MatrixViewData struct {
    UserName  string
    Role      string
    IsAdmin   bool
    SiteID    string   // resolved site ("" = All locations, admin only)
    SiteName  string   // display name for the filter bar
    DateFrom  string   // YYYY-MM-DD
    DateTo    string   // YYYY-MM-DD
    Dates     []string // []string of YYYY-MM-DD, inclusive
    Rows      []MatrixRow
    Sites     []Site   // from loadSites(false) for the dropdown
    EventID   string   // optional event filter ("" = none)
    Events    []Event  // optional; only if an event filter is wired
}

type MatrixRow struct {
    IntakeID     string
    Name         string
    Cells        map[string]string // date(YYYY-MM-DD) -> status ("" = empty)
    TotalDays    int               // len(Dates)
    PresentCount int
    LastPresent  string            // YYYY-MM-DD or ""
    IsDropout    bool              // LastPresent older than 14 days
}

type MatrixCell struct {
    IntakeID string
    Date     string
    Status   string // "", "present", "absent", "excused", "walk_in"
    SiteID   string
    EventID  string
}
```

### 2. `handleMatrix` — build the view

Replace the skeleton body. Flow:

1. `u := s.currentSession(r)` (non-nil, guaranteed by `requireAuth`).
2. Parse query params: `site`, `from`, `to`, `event`. Validate `from`/`to` as `YYYY-MM-DD` via `time.Parse("2006-01-02", ...)`; on invalid, fall back to defaults.
3. **Defaults:** `to = time.Now().In(hst).Format("2006-01-02")`; `from = to - 13 days` (14-day window). Compute via `time.Now().In(hst).AddDate(0,0,-13)`.
4. **Resolve site** via `resolveSiteID(u, siteParam)`:
   - If `u.Role == "admin"`: accept `siteParam` if it matches an active site, else `""` (All locations); if `siteParam` empty and no explicit choice, default to first active site from `loadSites(false)`.
   - If `u.Role == "case_manager"`: **ignore** `siteParam` entirely. Query intakes with filter `assigned_to='<u.ID>'` (escaped), count by `site` relation, pick the most common; fallback to first active site. Store in `SiteID`.
5. **Cap range:** if `(to - from) > 30 days`, set `to = from + 29 days`.
6. `dates := buildDateRange(from, to)`.
7. `rows := s.loadMatrixRows(u, siteID, dates, eventID)`.
8. `sites := must(s.loadSites(false))` for the dropdown (admin only shows the select; case_manager shows a static label).
9. Execute `s.tpl.ExecuteTemplate(w, "matrix", view)`.

### 3. `buildDateRange(from, to string) []string`

Loop from `from` to `to` inclusive, `AddDate(0,0,1)` each step, format `2006-01-02`. Return the slice.

### 4. `loadMatrixRows` — participants + attendance map

- **Participants:** query `intake` collection via `s.pb.FindRecordsByFilter(col.Id, filter, "name", 1000, 0)` where `filter` is:
  - case_manager: `assigned_to='<u.ID>'` (escaped) — their site's participants.
  - admin with `siteID != ""`: `site='<siteID>'` (escaped).
  - admin with `siteID == ""` (All locations): `1=1`.
  - Optionally append `&& status!='completed'` if desired (keep simple: no status filter unless specified).
- **Attendance map:** query `attendance` collection with filter `site='<siteID>' && date>='<from>' && date<='<to>'` (plus `&& event='<eventID>'` if event filter active). Build `map[intakeID]map[date]status`.
- **Build rows:** for each participant, `Name = rec.GetString("name")`, `IntakeID = rec.Id`, `Cells` initialized to `""` for every date, then filled from the attendance map. Compute `PresentCount` (count of `"present"`), `LastPresent` (max date with `"present"`), `IsDropout = LastPresent != "" && LastPresent < (to - 13 days)`.

### 5. `cycleStatus(current string) string`

`"" → "present" → "absent" → "excused" → "walk_in" → ""`. Implement as a switch or ordered slice lookup.

### 6. `handleToggle` — HTMX POST + no-JS fallback

Route: `mux.HandleFunc("/attendance/toggle", s.requireAuth(s.handleToggle))` in `server.go`.

1. Parse form values: `intake_id`, `date`, `site_id`, `event_id` (optional). Validate `date` format and that `intake_id`/`site_id` are non-empty.
2. **Authorization:** if `u.Role == "case_manager"`, verify the intake's `assigned_to == u.ID`; else 403. (Admin passes.)
3. **Find existing record:** query `attendance` with filter `intake='<intake_id>' && date='<date>'` (plus `&& event='<event_id>'` if event filter active). Use `s.pb.FindRecordsByFilter` with limit 1.
4. **Cycle:** `next := cycleStatus(existingStatus)`.
5. **Apply:**
   - If `next == ""` and a record exists → `s.pb.Delete(rec)`.
   - If `next != ""` and no record → create new record via `s.pb.NewRecord(col)`; set `intake`, `site`, `date`, `status = next`, `recorded_by = u.ID`, `check_in_time = time.Now().In(hst).Format("2006-01-02 15:04:05")`, and `event` if event filter active. `s.pb.Save(rec)`.
   - If `next != ""` and record exists → set `status = next`, `s.pb.Save(rec)`.
6. **Response:**
   - If request is HTMX (`HX-Request` header present) → return the `{{define "matrix-cell"}}` fragment via `s.tpl.ExecuteTemplate(w, "matrix-cell", cell)` with `Content-Type: text/html`.
   - Else (no-JS) → `http.Redirect(w, r, matrixRedirectURL(siteID, from, to, eventID), http.StatusSeeOther)` (303). The redirect URL preserves the current filter query params so the page reloads with the updated cell.

### 7. `renderMatrixCell` / `matrixRedirectURL`

- `renderMatrixCell` builds a `MatrixCell` and executes the `matrix-cell` template.
- `matrixRedirectURL(siteID, from, to, eventID)` returns `/attendance?site=...&from=...&to=...&event=...` (omit empty params).

### 8. Template — `{{define "matrix"}}` (replace body in `index.html`)

Keep the existing topbar block. Replace the `<div class="container container-admin">` body:

- **Filter bar** (`.inline-form`):
  - Site `<select class="field-input" name="site">` — admin only, options from `.Sites` plus an "All locations" option (`value=""`); case_manager renders a static `<span class="muted">` label instead.
  - `<input type="date" class="field-input" name="from" value="{{.DateFrom}}">` and `<input type="date" class="field-input" name="to" value="{{.DateTo}}">`.
  - `<button type="submit" class="btn btn-primary">Apply</button>`.
  - Wrap in `<form method="get" action="/attendance" class="inline-form">` so Apply updates the URL to `?site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD`.
- **Matrix table** (`.matrix-wrap` scroll container → `.matrix-table`):
  - `<table class="matrix-table">` with `<thead>`: first `<th class="matrix-name-col">Participant</th>`, then one `<th>` per date (`.matrix-date-col`, show `MM/DD`), then `<th class="matrix-total-col">Total</th>`.
  - `<tbody>`: for each `.Rows`, a `<tr>` (add `.row-dropout` class when `IsDropout`):
    - `<td class="matrix-name-col">` — sticky name (`.matrix-name`), plus a `.status-badge` "STOPPED" when `IsDropout`.
    - One `<td class="matrix-cell-col">` per date, each rendering the `{{define "matrix-cell"}}` partial.
    - `<td class="matrix-total-col">` — `<span class="matrix-total-badge">{{.PresentCount}}/{{.TotalDays}}</span>`.
  - Empty state: if no rows, render `<p class="empty-state">No participants for this filter.</p>`.

### 9. Template — `{{define "matrix-cell"}}` (new partial at end of `index.html`)

A single cell that works for both HTMX swap and initial render. It is a `<form method="post" action="/attendance/toggle" class="matrix-cell-form">` with hidden inputs `intake_id`, `date`, `site_id`, `event_id`, and HTMX attributes:

```html
{{define "matrix-cell"}}
<form method="post" action="/attendance/toggle" class="matrix-cell-form"
      hx-post="/attendance/toggle" hx-target="closest form" hx-swap="outerHTML"
      hx-trigger="click" hx-include="closest form">
  <input type="hidden" name="intake_id" value="{{.IntakeID}}">
  <input type="hidden" name="date" value="{{.Date}}">
  <input type="hidden" name="site_id" value="{{.SiteID}}">
  {{if .EventID}}<input type="hidden" name="event_id" value="{{.EventID}}">{{end}}
  <button type="submit" class="matrix-dot dot-{{.Status}}" aria-label="Toggle status for {{.Date}}"></button>
</form>
{{end}}
```

- `hx-trigger="click"` on the form makes the whole cell clickable; `hx-target="closest form"` + `hx-swap="outerHTML"` replaces just that cell with the returned fragment.
- **No-JS fallback:** without HTMX, the native `<form method="post">` submits to `/attendance/toggle`; the handler returns a 303 redirect back to `/attendance` with the same query params, reloading the page with the updated cell.
- The dot button is styled by status via `dot-{{.Status}}` (empty → `dot-` with no color).

### 10. CSS — `r3-intake/internal/assets/public/app.css`

Add a matrix section (bump the `?v=` cache-buster in the matrix template's stylesheet link):

- `.matrix-wrap { overflow-x: auto; border: 1px solid #e4d9c8; border-radius: 10px; background: #fffdfa; }`
- `.matrix-table { border-collapse: collapse; width: 100%; font: 14px 'Public Sans', sans-serif; }`
- `.matrix-table th, .matrix-table td { padding: 8px 10px; border-bottom: 1px solid #eee3d2; text-align: center; }`
- `.matrix-name-col { position: sticky; left: 0; background: #fffdfa; z-index: 2; text-align: left; min-width: 200px; box-shadow: 1px 0 0 #e4d9c8; }`
- `.matrix-date-col { min-width: 44px; color: #6b5f52; font-size: 12px; }`
- `.matrix-total-col { min-width: 70px; font-weight: 600; }`
- `.matrix-total-badge { background: #f7f1e6; border: 1px solid #e4d9c8; border-radius: 999px; padding: 2px 10px; font-size: 12px; }`
- `.matrix-cell-form { margin: 0; display: inline-block; }`
- `.matrix-dot { width: 18px; height: 18px; border-radius: 50%; border: 1px solid #e4d9c8; background: transparent; cursor: pointer; padding: 0; }`
- `.matrix-dot.dot-present { background: #3f6b34; border-color: #3f6b34; }`
- `.matrix-dot.dot-absent { background: #eee; border-color: #d9cbb6; }`
- `.matrix-dot.dot-excused { background: #8a6a1e; border-color: #8a6a1e; }`
- `.matrix-dot.dot-walk_in { background: #2a4d8a; border-color: #2a4d8a; }`
- `.matrix-dot.dot- { background: transparent; }` (empty = no dot)
- `.matrix-table tr.row-dropout td { background: #fdf2f1; }`
- `.matrix-name { font-weight: 600; color: #2b2320; }`
- `.matrix-table tr.row-dropout .matrix-name { color: #8f3a2e; }`

### 11. Route registration — `r3-intake/internal/server/server.go`

Add immediately after the existing `/attendance` registration:

```go
mux.HandleFunc("/attendance/toggle", s.requireAuth(s.handleToggle))
```

## Verification Criteria

1. **Build/vet/test:** `go build ./...`, `go vet ./...`, `go test ./...` all pass from the repo root.
2. **Template parse:** `s.tpl` parses successfully in `New()` — both `{{define "matrix"}}` and `{{define "matrix-cell"}}` are registered; no undefined-function or malformed-block errors.
3. **Matrix render:** `GET /attendance` renders the grid with:
   - Sticky participant-name column (`.matrix-name-col` with `position: sticky; left: 0; background: #fffdfa`).
   - Date header columns (last 14 days by default, HST).
   - Per-participant Total column showing `PresentCount/TotalDays` badge.
   - Status dots colored per spec (present `#3f6b34`, absent `#eee`+border, excused `#8a6a1e`, walk_in `#2a4d8a`, empty = no dot).
   - Dropout rows highlighted (`.row-dropout` + "STOPPED" badge) when `LastPresent` is older than 14 days.
   - Filter bar with site select (admin: any active site or "All locations"; case_manager: static label), from/to date inputs, Apply button; URL updates to `?site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD`.
4. **Site scoping:** case_manager sees only their assigned site's participants and cannot change site; admin can select any active site or "All locations".
5. **Date cap:** a `from`/`to` span > 30 days is truncated to 30 days from the start date.
6. **Cell toggle (HTMX):** clicking a cell cycles empty → present → absent → excused → walk_in → empty; the cell swaps in place via the returned `matrix-cell` fragment; no full page reload.
7. **No-JS fallback:** with HTMX disabled, the native POST to `/attendance/toggle` returns a 303 redirect back to `/attendance` with the same query params; the page reloads showing the updated cell.
8. **Uniqueness:** toggling never creates duplicate attendance records for the same `(intake, date)` or `(event, intake, date)`; create sets `status=present`, `recorded_by=<current user>`, `check_in_time=<current HST time>`; cycling to empty deletes the record.
9. **Authorization:** a case_manager cannot toggle a cell for an intake not assigned to them (403).
