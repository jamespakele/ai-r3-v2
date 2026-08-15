# Working Plan: Fix attendance matrix date handling and remove STOPPED heuristic

## Objective
Rework the attendance matrix screen so the **event** is the single source of truth for the matrix date range, and remove the "STOPPED" dropout heuristic entirely.

Concretely:
1. Remove the `from`/`to` date input boxes and the `Apply` button from the matrix filter form — the event dropdown alone drives the date range.
2. Confirm the grid header, stat cards, and toggle/walk-in forms all use the event-scoped `dates` (this already happens automatically once the date inputs are gone, because `explicitRange` in `parseMatrixFilters` will always be `false` and the existing auto-scope block applies the effective event's start/end dates).
3. Show the year in the event-dates label and in the grid column headers.
4. Remove the `IsDropout` field, its computation, the `Stopped` summary counter, the `row-dropout` CSS, the `STOPPED` badge, and the `Stopped` stat card. Matrix rows become uniform attendance ticks only.

## Constraints
- **Language:** Go (server), Go `html/template` (server-rendered), HTMX + Alpine.js, vanilla CSS.
- **Framework:** R3 Intake — Go server with embedded PocketBase. No new dependencies.
- **Time zone:** All timestamps in HST (not directly relevant to this change, but do not alter date parsing semantics).
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`. Keep the existing visual language.
- **Real template path:** `r3-intake/internal/assets/public/index.html` (the card's "index.html" is shorthand — do NOT touch a top-level `r3-intake/index.html`).
- **Build/test gate:** `cd r3-intake && go build ./... && go vet ./... && go test ./...` must pass.
- Implementation is done by the `omp` coding agent in one coherent pass; the plan must be precise enough to implement without re-deriving decisions.

## File Structure
Files to **modify** (all under the worktree root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_ee9dda74`):

- `r3-intake/internal/server/attendance.go` — remove dropout logic, `IsDropout` field, `Stopped` summary field.
- `r3-intake/internal/assets/public/index.html` — remove date inputs/Apply button, STOPPED badge, Stopped stat card; add year to date labels.
- `r3-intake/internal/assets/public/app.css` — remove `row-dropout` rules; remove `.matrix-table .status-badge` block; remove `.stat-red` (only used by the Stopped card).
- `r3-intake/internal/server/attendance_test.go` — update `MatrixSummary` literals and stat-card label assertions.
- `r3-intake/internal/server/attendance_stats_integration_test.go` — remove `"Stopped"` from the stat-card label list.

No new files.

## Implementation Notes

### attendance.go
- **`MatrixRow` struct (lines ~50-56):** remove the `IsDropout bool` field. Keep `LastPresent string` (still used to track the last present date — verify it is not otherwise dead; it is only consumed by the dropout logic, so if nothing else reads it after this change, it may be removed too, but keeping it is harmless and lower-risk — **decision: keep `LastPresent`** unless `go vet`/build flags it as unused, in which case remove it).
- **`MatrixSummary` struct (lines ~60-64):** remove the `Stopped int` field. Keep `TotalCheckIns`, `ActiveParticipants`, `AvgRate`.
- **Threshold block (lines ~398-400):** delete the three lines that compute `threshold`, `threshold.AddDate(0,0,-13)`, and `thresholdStr`. This block becomes dead once line 440 is removed.
- **Line 440:** delete `row.IsDropout = row.LastPresent != "" && row.LastPresent < thresholdStr`.
- **`computeSummary` (lines ~455-465):** remove the `if row.IsDropout { s.Stopped++ }` branch. The loop still computes `TotalCheckIns`, `ActiveParticipants`, and `AvgRate` from the rows.
- **`parseMatrixFilters`:** no change needed. The auto-scope block (already present) applies the effective event's `StartDate`/`EndDate` whenever `explicitRange` is false. With the date inputs removed from the form, `explicitRange` is always false, so the event span always applies. Do not remove the auto-scope block — it is now the sole path for setting `from`/`to`.

### index.html (matrix-content template, lines ~1167-1265)
- **Filter form (lines ~1169-1180):** remove the two `<input type="date" ... name="from" ...>` and `name="to" ...>` fields (lines 1176-1177) and the `<button type="submit" class="btn btn-primary">Apply</button>`. Keep the event `<select name="event">` and the site muted span. The form still submits via `hx-get="/attendance"` on `change, submit` — with only the event select, changing the event re-fetches the matrix. Keep the form element itself (it still carries the event param).
- **Walk-in panel (lines ~1185-1214):** **KEEP** the hidden `<input type="hidden" name="from" value="{{.DateFrom}}">` and `name="to" value="{{.DateTo}}">` in BOTH the `walkin-search-filters` div and the `walkin-create` form. These carry the event-scoped range to the walk-in handlers and must remain.
- **Toggle/walk-in forms (lines ~1273-1287):** **KEEP** the hidden `name="from"`/`name="to"` inputs (they use `{{.From}}`/`{{.To}}`). These carry the event-scoped range to the toggle/walk-in handlers.
- **Event dates label (lines 1217-1219):** change the format from `MM/DD – MM/DD` to include the year, e.g. `Mar 1 – Apr 15, 2026`. Go templates have no built-in date formatting, so implement via a small helper or a precomputed template field. **Recommended:** add a template function or precompute formatted strings in the handler (e.g. `EventStartLabel`, `EventEndLabel`) using `time.Parse("2006-01-02", ...)` + `time.Format("Jan 2, 2006")` for the end and `Jan 2` for the start (or `Jan 2, 2006` for both). If adding a template func is undesirable, precompute the two label strings in the Go handler and reference them in the template. The label should read like `Event dates: Mar 1 – Apr 15, 2026`.
- **Grid header (line 1227):** change `{{range .Dates}}<th class="matrix-date-col">{{slice . 5 7}}/{{slice . 8 10}}</th>{{end}}` to a compact format that includes the year, e.g. `3/1/26`. Since `.Dates` is a `[]string` of `YYYY-MM-DD`, either (a) precompute a parallel `[]string` of formatted labels in the handler and range over that, or (b) add a template function `fmtDate` that parses and reformats. **Recommended:** precompute a `DatesLabel []string` in the handler (or a `dateLabel` template func) so the template stays simple. Format: `M/D/YY` (e.g. `3/1/26`).
- **Row (line 1233):** change `<tr class="{{if .IsDropout}}row-dropout {{end}}">` to a plain `<tr>` (no conditional class).
- **STOPPED badge (line 1236):** remove the `{{if .IsDropout}}<span class="status-badge">STOPPED</span>{{end}}` line entirely.
- **Stopped stat card (line 1261):** remove the `<div class="stat-card"><div class="stat-number stat-red">{{.Summary.Stopped}}</div><div class="stat-label">Stopped</div></div>` line. The remaining stat cards (Total check-ins, Active participants, Avg attendance rate) stay.

### app.css
- **Lines 305, 307, 309:** remove the three `.matrix-table tr.row-dropout ...` rules.
- **Lines 313-319:** remove the `.matrix-table .status-badge { ... }` block. **Verified:** the base `.status-badge` (line 203) is used elsewhere (event status badges at index.html lines 605, 705, 1354), so **keep** the base `.status-badge` and the `status-*`/`event-status-*` variants. Only the matrix-specific `.matrix-table .status-badge` block (used solely by the STOPPED badge) is removed.
- **Line 344:** remove `.stat-red { color: #8f3a2e; }`. **Verified:** `stat-red` is used only by the Stopped stat card (index.html line 1261), which is being removed. Safe to delete.

### Tests
- **`attendance_test.go`:**
  - Line 33: `MatrixSummary{TotalCheckIns: 3, ActiveParticipants: 1, Stopped: 0, AvgRate: 40}` → remove `Stopped: 0`.
  - Lines 36-43: the `"dropout and average rounds down"` test case uses `{PresentCount: 0, IsDropout: true}` and `want: MatrixSummary{... Stopped: 1 ...}`. **Remove this test case entirely** (the dropout concept no longer exists). If the "average rounds down" behavior is worth preserving, split it into a separate case without `IsDropout`; otherwise drop it.
  - Line 50: `MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 0, Stopped: 0, AvgRate: 0}` → remove `Stopped: 0`.
  - Lines 101, 225, 287: `MatrixSummary{... Stopped: 0 ...}` literals → remove `Stopped: 0`.
  - Line 111: the stat-card label assertion list `"Total check-ins", "Active participants", "Stopped", "Avg attendance rate"` → remove `"Stopped"`.
- **`attendance_stats_integration_test.go` line 64:** the label list `"Total check-ins", "Active participants", "Stopped", "Avg attendance rate"` → remove `"Stopped"`.

## Logical Consequences
Every downstream site found and the decision for each:

| Site | Location | Decision |
|---|---|---|
| `from`/`to` date inputs + Apply button | index.html 1176-1178 | **remove** |
| Event `<select name="event">` | index.html ~1171 | **keep** (now sole driver) |
| Hidden `from`/`to` in walkin-search-filters | index.html 1192-1193 | **keep** (carries event-scoped range) |
| Hidden `from`/`to` in walkin-create form | index.html 1203-1204 | **keep** (carries event-scoped range) |
| Hidden `from`/`to` in toggle/walk-in forms | index.html 1273-1274, 1286-1287 | **keep** (carries event-scoped range) |
| Event dates label (MM/DD) | index.html 1218 | **change** → include year (`Mar 1 – Apr 15, 2026`) |
| Grid header (MM/DD) | index.html 1227 | **change** → compact with year (`3/1/26`) |
| Row class `row-dropout` conditional | index.html 1233 | **remove** conditional → plain `<tr>` |
| STOPPED badge | index.html 1236 | **remove** |
| Stopped stat card | index.html 1261 | **remove** |
| `IsDropout` field | attendance.go 56 | **remove** |
| `LastPresent` field | attendance.go 55 | **keep** (harmless; remove only if build flags unused) |
| `Stopped` summary field | attendance.go 64 | **remove** |
| threshold/thresholdStr block | attendance.go 398-400 | **remove** (dead after line 440 removal) |
| `row.IsDropout` computation | attendance.go 440 | **remove** |
| `s.Stopped++` branch | attendance.go 460-461 | **remove** |
| `parseMatrixFilters` auto-scope block | attendance.go ~190-205 | **keep** (now the sole date-range path) |
| `.matrix-table tr.row-dropout` rules | app.css 305, 307, 309 | **remove** |
| `.matrix-table .status-badge` block | app.css 313-319 | **remove** (matrix-only) |
| Base `.status-badge` + `status-*`/`event-status-*` | app.css 203+ | **keep** (used by event status badges) |
| `.stat-red` | app.css 344 | **remove** (only used by Stopped card) |
| `MatrixSummary{... Stopped ...}` literals | attendance_test.go 33, 50, 101, 225, 287 | **change** → drop `Stopped` |
| `"dropout and average rounds down"` test case | attendance_test.go 36-43 | **remove** (or split to keep the rounding assertion) |
| `"Stopped"` label assertion | attendance_test.go 111 | **remove** |
| `"Stopped"` label assertion | attendance_stats_integration_test.go 64 | **remove** |

## Verification Criteria
1. **Build/vet/test:** `cd r3-intake && go build ./... && go vet ./... && go test ./...` passes with no references to `IsDropout`, `Stopped`, `thresholdStr`, `row-dropout`, or the matrix `status-badge` remaining. Grep the repo for these tokens to confirm zero hits (excluding the base `.status-badge`/`status-*` used by event badges).
2. **Event-scoped grid:** Selecting an event renders the grid header with exactly that event's start→end date span, and every column header shows the year (e.g. `3/1/26`).
3. **No date filter UI:** The matrix filter form shows only the event dropdown (and site muted span) — no `from`/`to` date inputs and no `Apply` button.
4. **No STOPPED UI:** No `row-dropout` red row highlighting, no `STOPPED` badge, and no `Stopped` stat card. Stat cards show only Total check-ins, Active participants, and Avg attendance rate.
5. **Event dates label:** The label reads like `Event dates: Mar 1 – Apr 15, 2026` (year present).
6. **Walk-in/toggle forms:** After removing the date inputs, the walk-in search/create forms and the toggle/walk-in forms still submit the correct event-scoped `from`/`to` hidden values (i.e. the event's start/end dates), verified by inspecting the rendered HTML and/or the walk-in handler receiving the event's range.
7. **Single source of truth:** With no date inputs, changing the event dropdown re-fetches the matrix scoped to the new event's dates (HTMX `change` trigger on the select).
