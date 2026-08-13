# Working Plan: Summary Stats Cards & Attendance Filters (Story 1.5)

## Objective

Add four aggregate summary stat cards below the attendance matrix, a program/event filter dropdown, and HTMX-driven dynamic re-rendering so that changing any filter (site, date range, event) updates both the matrix grid and the stat cards without a full page reload.

The four cards (in a flex row below the matrix):
- **Total check-ins** — green `#3f6b34` — count of all `present` + `walk_in` records in the current date range
- **Active participants** — accent `#b5502e` — count of participants with ≥ 1 `present` record in range
- **Stopped** — red `#8f3a2e` — count of participants flagged as dropouts (>14 days since last present)
- **Avg attendance rate** — yellow `#8a6a1e` — `total present / (participants × days in range)` as a percentage

## Constraints

- **Language:** Go (server-rendered templates), embedded PocketBase v0.39 JS migration API (camelCase, no `app.dao()`), HTMX + Alpine.js, vanilla CSS.
- **No new dependencies.** Reuse existing `s.pb.FindRecordsByFilter`, `s.pb.FindCollectionByNameOrId`, `mcpmod.EscapeFilter`, `hst` timezone, and the existing `matrix`/`matrix-cell` templates.
- **All timestamps in HST** (UTC-10, no DST). `hst` is already defined in `admin.go`.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii. Colors are hardcoded hex in `app.css` (no `:root` vars exist — follow that convention).
- **Scope tight:** filter-driven dynamic updates + stat cards + event dropdown only. Toggle-driven stat refresh (after a cell toggle) is **out of scope** for this card.

## File Structure

| File | Action | Change |
|------|--------|--------|
| `r3-intake/internal/server/attendance.go` | **Modify** | Add `Event` struct, `MatrixSummary` struct, `WalkInCount` field on `MatrixRow`, `Events` + `Summary` fields on `MatrixViewData`; add `loadEvents()`; add `computeSummary()`; update `handleMatrix` to load events, compute summary, and render partial on HTMX requests. |
| `r3-intake/internal/assets/public/index.html` | **Modify** | Extract filter form + matrix + stats into `{{define "matrix-content"}}` partial wrapped in `<div id="matrix-and-stats">`; add event dropdown to filter form; add `{{define "stat-cards"}}` partial; make `matrix` template include the partial; add HTMX attributes to the filter form. |
| `r3-intake/internal/assets/public/app.css` | **Modify** | Add `.stat-cards`, `.stat-card`, `.stat-number`, `.stat-label` styles + color classes; ensure 2×2 wrap at 375px. |
| `r3-intake/internal/server/server.go` | **No change** | Routes already exist (`/attendance`, `/attendance/toggle`). |
| `r3-intake/pocketbase/migrations/007_events_attendance.js` | **No change** | `events` collection already exists. |

## Implementation Notes

### 1. Summary stats — compute from the already-loaded rows

Compute the summary from the `rows` returned by `loadMatrixRows` (already filtered by site/date/event and site-scoped for case managers). This guarantees the cards always match the matrix exactly and avoids a second query.

- **Add `WalkInCount int` to `MatrixRow`.** `loadMatrixRows` currently only increments `PresentCount` for `status == "present"`. Add a parallel increment for `status == "walk_in"` so the summary can count walk-ins without re-scanning cells.
- **Add a `MatrixSummary` struct** (mirrors the `MatrixRow`/`MatrixViewData` naming convention):
  ```go
  type MatrixSummary struct {
      TotalCheckIns      int
      ActiveParticipants int
      Stopped            int
      AvgRate            int // 0–100, rounded
  }
  ```
- **Add `computeSummary(rows []MatrixRow, days int) MatrixSummary`**:
  - `TotalCheckIns` = Σ(`PresentCount` + `WalkInCount`) across rows.
  - `ActiveParticipants` = count of rows with `PresentCount >= 1`.
  - `Stopped` = count of rows with `IsDropout == true`.
  - `AvgRate` = `totalPresent / (len(rows) × days) × 100`, rounded. **Guard division by zero:** if `len(rows) == 0 || days == 0`, return `0`.
- **Add `Summary MatrixSummary` and `Events []Event` to `MatrixViewData`.** Populate in `handleMatrix` after `loadMatrixRows` and `loadSites`.

### 2. Program/event filter dropdown

- **Add an `Event` struct** (mirrors `Site`):
  ```go
  type Event struct {
      ID        string
      Name      string
      SiteID    string
      StartDate string
      EndDate   string
      Status    string
  }
  ```
- **Add `loadEvents(siteID string) ([]Event, error)`** using `s.pb.FindCollectionByNameOrId("events")` and `FindRecordsByFilter`. Filter `status='active'` (optionally include `completed`); when `siteID != ""`, add `&& site='<siteID>'` so case managers only see events for their resolved site. Sort by `start_date,name`.
- **Add `Events []Event` to `MatrixViewData`.** In `handleMatrix`, call `loadEvents(siteID)` (the resolved site) and populate.
- **Template:** add an event `<select name="event">` to the filter form, before the site select. First option: `All dates — no event filter` (value `""`). Render `{{range .Events}}<option value="{{.ID}}" {{if eq $.EventID .ID}}selected{{end}}>{{.Name}}</option>{{end}}`. Show the dropdown to **all users** (admins see all active events; case managers see only their site's events via `loadEvents(siteID)`). The `eventID` is already threaded through `loadMatrixRows` and `handleToggle`, so no handler logic change is needed for filtering.

### 3. HTMX dynamic updates — extract a `matrix-content` partial

The current filter form is a plain GET form (full reload). To make filter changes dynamic:

- **Extract** the filter form + matrix table + stat cards into a new `{{define "matrix-content"}}` partial, wrapped in a single container:
  ```html
  <div id="matrix-and-stats">
    <form method="get" action="/attendance" class="inline-form"
          hx-get="/attendance" hx-target="#matrix-and-stats" hx-swap="outerHTML"
          hx-trigger="change, submit">
      ... event select, site select, from/to inputs, Apply button ...
    </form>
    {{if .Rows}} ... matrix-wrap table ... {{else}} empty-state {{end}}
    {{template "stat-cards" .}}
  </div>
  ```
- **`matrix` template** keeps the full page shell (topbar, container, section title) and just `{{template "matrix-content" .}}` in place of the current inline form + matrix block.
- **`handleMatrix` must detect HTMX:** at the end, branch on `r.Header.Get("HX-Request") == "true"` (same pattern as `handleToggle`). If HTMX, `ExecuteTemplate(w, "matrix-content", view)`; otherwise `ExecuteTemplate(w, "matrix", view)`. This keeps the no-JS full-page fallback working (form still has `method="get" action="/attendance"`).
- **`hx-trigger="change, submit"`** re-renders on any select/date change and on Apply. Because the target is `#matrix-and-stats` with `outerHTML`, the form itself is replaced with the re-rendered version carrying the new filter values — no stale state.
- **`stat-cards` partial** (rendered inside `matrix-content`, below the matrix):
  ```html
  {{define "stat-cards"}}
  <div class="stat-cards">
    <div class="stat-card"><div class="stat-number stat-green">{{.Summary.TotalCheckIns}}</div><div class="stat-label">Total check-ins</div></div>
    <div class="stat-card"><div class="stat-number stat-accent">{{.Summary.ActiveParticipants}}</div><div class="stat-label">Active participants</div></div>
    <div class="stat-card"><div class="stat-number stat-red">{{.Summary.Stopped}}</div><div class="stat-label">Stopped</div></div>
    <div class="stat-card"><div class="stat-number stat-yellow">{{.Summary.AvgRate}}%</div><div class="stat-label">Avg attendance rate</div></div>
  </div>
  {{end}}
  ```

### 4. Stat card CSS

Add to `app.css` (after the matrix block, ~line 283), following the existing hardcoded-hex convention:
```css
.stat-cards { display: flex; flex-wrap: wrap; gap: 12px; margin: 16px 0 0; }
.stat-card { flex: 1 1 160px; min-width: 140px; background: #fffdfa; border: 1px solid #e4d9c8; border-radius: 14px; padding: 14px 16px; }
.stat-number { font: 600 26px 'Lora', serif; line-height: 1.1; }
.stat-label { font: 13px 'Public Sans', sans-serif; color: #6b5f52; margin-top: 4px; }
.stat-green  { color: #3f6b34; }
.stat-accent { color: #b5502e; }
.stat-red    { color: #8f3a2e; }
.stat-yellow { color: #8a6a1e; }
```
`flex: 1 1 160px` with `flex-wrap` yields 4-across on wide screens and **2×2 at 375px** (two ~160px cards per row) automatically — no media query needed, but add one if the 375px measurement shows 1-per-row. Each card keeps its colored number + muted label.

### Edge cases

- **No attendance records in range:** all rows have `PresentCount == 0`, so `TotalCheckIns = 0`, `ActiveParticipants = 0`, `Stopped = 0`, and `AvgRate` guards `len(rows) == 0` → `0%`.
- **Division by zero for avg rate:** guard `len(rows) == 0 || days == 0` before dividing.
- **No events created:** dropdown shows only the `All dates — no event filter` option; `loadEvents` returns empty slice, `{{range}}` renders nothing.
- **Case-manager site scoping:** `loadMatrixRows` already scopes rows to the case manager's site, so stats computed from rows are automatically site-scoped. `loadEvents(siteID)` scopes the event dropdown the same way.
- **`slice` template func:** the matrix template already uses `{{slice . 5 7}}` — this is a Go template **builtin**, not a registered func, so no change needed.
- **Toggle staleness:** after a cell toggle, the Total badge and stat cards would be stale. This is explicitly **out of scope** (the card is filter-driven updates only).

## Verification Criteria

1. **Build & vet:** `cd r3-intake && go build ./... && go vet ./...` — no errors.
2. **Tests:** `go test ./...` — no existing tests; add a small unit test for `computeSummary` covering: mixed present/walk_in counts, active-participant counting, dropout counting, and the zero/division-by-zero guard (empty rows → all 0 / 0%).
3. **Template parse + render:** `go build` already parses the embedded template (parse failure fails the build). Additionally render `matrix-content` and `stat-cards` with a populated `MatrixViewData` to confirm no missing-field errors.
4. **Manual reasoning against AC:**
   - Four cards render in a flex row below the matrix with the correct colors (`#3f6b34`, `#b5502e`, `#8f3a2e`, `#8a6a1e`) and labels.
   - `TotalCheckIns` = present + walk_in records in range; `ActiveParticipants` = rows with ≥1 present; `Stopped` = rows with `IsDropout`; `AvgRate` = `totalPresent / (len(rows) × days)`.
   - Changing site/from/to/event re-renders `#matrix-and-stats` via HTMX (no full reload) and recalculates cards from the filtered rows.
   - Empty filter range → all cards `0` / `0%`.
   - At 375px, cards wrap to 2×2 with colored numbers + muted labels.
   - No-JS fallback still works: form `method="get" action="/attendance"` performs a full-page reload when HTMX is absent.
